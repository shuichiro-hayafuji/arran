import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/network/api_client.dart';
import '../../session/domain/session.dart';
import 'dto/login_response_dto.dart';

final loginRepositoryProvider = Provider.autoDispose<LoginRepository>(
  (ref) => LoginRepositoryImpl(ref.watch(publicApiClientProvider)),
);

abstract interface class LoginRepository {
  Future<Session> login(String username, String password);
}

class LoginRepositoryImpl implements LoginRepository {
  LoginRepositoryImpl(this._apiClient);
  final ApiClient _apiClient;

  @override
  Future<Session> login(String username, String password) async {
    try {
      final response = await _apiClient.post(
        '/auth/login',
        data: {'username': username.trim(), 'password': password},
      );
      final session = LoginResponseDto.fromJson(
        response.data as Map<String, dynamic>,
      ).toDomain();
      if (!session.alive) throw StateError('ログイン応答が無効です。');
      return session;
    } on ApiException catch (error) {
      if (error.statusCode == 404) {
        throw const ApiException(
          '接続先にログインAPIがありません。Goサーバーの更新・再起動と接続先を確認してください。',
          statusCode: 404,
        );
      }
      if (error.statusCode == 503) {
        throw const ApiException(
          'サーバーの認証処理に失敗しました。DB接続とGoサーバーの状態を確認してください。',
          statusCode: 503,
        );
      }
      rethrow;
    }
  }
}
