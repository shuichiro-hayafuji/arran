import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../config/env.dart';
import '../../../core/network/api_error_handler.dart';
import '../../../core/network/api_exception.dart';
import '../domain/session.dart';
import '../repository/dto/stored_session_dto.dart';

/// セッションの永続化を抽象化し、端末の安全なストレージとテスト用実装を差し替える。
abstract class SessionStorage {
  Future<String?> read();
  Future<void> write(String value);
  Future<void> delete();
}

class SecureSessionStorage implements SessionStorage {
  static const _storage = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
    iOptions: IOSOptions(
      accessibility: KeychainAccessibility.unlocked_this_device,
    ),
  );
  // 接続先ごとに保存先を分け、別環境へ同じ認証情報を持ち出さない。
  String get _key => 'arran.session.v1.${Env.apiBaseUrl}';
  @override
  Future<String?> read() => _storage.read(key: _key);
  @override
  Future<void> write(String value) => _storage.write(key: _key, value: value);
  @override
  Future<void> delete() => _storage.delete(key: _key);
}

final sessionControllerProvider = Provider<SessionController>(
  (ref) => authSession,
);

final authSession = SessionController(SecureSessionStorage());

/// セッションの復元・期限切れ・失効を管理し、ルーターと画面へ認証状態を通知する。
class SessionController extends ChangeNotifier {
  SessionController(this.storage, {Dio? dio})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              baseUrl: Env.apiBaseUrl,
              connectTimeout: const Duration(seconds: 10),
              receiveTimeout: const Duration(seconds: 20),
              sendTimeout: const Duration(seconds: 20),
              followRedirects: false,
            ),
          );

  final SessionStorage storage;
  final Dio _dio;
  Session? _session;
  Timer? _expiryTimer;
  bool initialized = false;
  String? storageError;

  /// 認証状態の切替時に増やし、ProviderScope 内の前セッションの状態を破棄する。
  int generation = 0;
  Future<void> _storageQueue = Future<void>.value();

  /// 保存と削除の完了順を保つ。失敗は呼び出し元へ返し、後続の処理は継続する。
  Future<void> _persist(Future<void> Function() action) {
    final operation = _storageQueue.then((_) => action());
    _storageQueue = operation.then<void>((_) {}, onError: (Object _) {});
    return operation;
  }

  String? get token => _session?.alive == true ? _session!.token : null;
  bool get loggedIn => token != null;

  Future<void> initialize() async {
    try {
      final saved = await storage.read();
      if (saved != null) {
        final session = StoredSessionDto.fromJson(
          jsonDecode(saved) as Map<String, dynamic>,
        );
        if (session.alive) {
          _session = session;
          _scheduleExpiry();
        } else {
          await storage.delete();
        }
      }
    } catch (_) {
      storageError = '認証情報を読み込めませんでした。再度ログインしてください。';
    }
    initialized = true;
    notifyListeners();
  }

  /// 認証済みセッションを保存し、アプリ全体へ公開する。
  Future<void> activate(Session session) async {
    if (!session.alive) {
      throw StateError('セッションが無効です。');
    }
    try {
      await _persist(
        () => storage.write(jsonEncode(StoredSessionDto.toJson(session))),
      );
    } catch (_) {
      // 保存失敗と、後始末の通信失敗を区別して利用者へ伝える。
      try {
        await _dio.post<dynamic>(
          '/auth/logout',
          options: Options(
            headers: {'Authorization': 'Bearer ${session.token}'},
          ),
        );
      } catch (_) {
        throw const ApiException(
          'ユーザー認証は成功しましたが、端末への認証情報の保存とサーバーのセッション解除に失敗しました。アプリを完全に終了して再起動してください。',
        );
      }
      throw const ApiException(
        'ユーザー認証は成功しましたが、端末に認証情報を保存できませんでした。アプリを完全に終了して再起動してください。',
      );
    }
    _session = session;
    storageError = null;
    generation++;
    _scheduleExpiry();
    notifyListeners();
  }

  /// 公開しないセッションを解除する（ログイン画面破棄後の遅延応答など）。
  Future<void> revoke(Session session) async {
    try {
      await _dio.post<dynamic>(
        '/auth/logout',
        options: Options(headers: {'Authorization': 'Bearer ${session.token}'}),
      );
    } on DioException catch (error) {
      if (error.response?.statusCode != 401) throw error.appException;
    }
  }

  Future<void> logout() async {
    final current = token;
    if (current == null) {
      await clearIfCurrent(_session?.token);
      return;
    }
    try {
      await _dio.post<dynamic>(
        '/auth/logout',
        options: Options(headers: {'Authorization': 'Bearer $current'}),
      );
    } on DioException catch (error) {
      if (error.response?.statusCode != 401) {
        throw error.appException;
      }
    }
    await clearIfCurrent(current);
  }

  /// 対象トークンが現在のセッションと一致する場合だけ失効させる。
  /// 以前のリクエストの遅れた 401 応答で、新しいログインを解除しないための照合。
  Future<void> clearIfCurrent(String? rejectedToken) async {
    if (_session?.token != rejectedToken) return;
    // ストレージの削除完了を待たず、認証が必要な画面を閉じる。
    _session = null;
    _expiryTimer?.cancel();
    generation++;
    notifyListeners();
    try {
      await _persist(storage.delete);
    } catch (_) {
      storageError = '端末の認証情報を削除できませんでした。';
    }
  }

  void _scheduleExpiry() {
    _expiryTimer?.cancel();
    final session = _session!;
    _expiryTimer = Timer(
      session.expiresAt.difference(DateTime.now()),
      () => unawaited(clearIfCurrent(session.token)),
    );
  }

  @override
  void dispose() {
    _expiryTimer?.cancel();
    _dio.close(force: true);
    super.dispose();
  }
}
