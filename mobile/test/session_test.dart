import 'package:spendable_today/core/network/api_client.dart';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/session/domain/session.dart';
import 'package:spendable_today/features/session/repository/dto/stored_session_dto.dart';
import 'package:spendable_today/features/session/provider/session_provider.dart';

class MemoryStorage implements SessionStorage {
  String? value;
  bool failWrite = false;
  @override
  Future<String?> read() async => value;
  @override
  Future<void> write(String newValue) async {
    if (failWrite) throw StateError('storage unavailable');
    value = newValue;
  }

  @override
  Future<void> delete() async {
    value = null;
  }
}

class AuthAdapter implements HttpClientAdapter {
  int logoutStatus = 204;
  int logoutCount = 0;
  String? authorization;
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? stream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path == '/auth/logout') {
      logoutCount++;
      authorization = options.headers['Authorization'] as String?;
      return ResponseBody.fromString(
        '{}',
        logoutStatus,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    throw StateError('Unexpected session request: ${options.path}');
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test('restores live sessions and removes expired sessions', () async {
    final storage = MemoryStorage();
    storage.value = jsonEncode(
      StoredSessionDto.toJson(
        Session(
          token: 'saved',
          expiresAt: DateTime.now().add(const Duration(hours: 1)),
        ),
      ),
    );
    final controller = SessionController(
      storage,
      apiClient: ApiClient(Dio()),
    );
    await controller.initialize();
    expect(controller.token, 'saved');
    controller.dispose();
    storage.value = jsonEncode(
      StoredSessionDto.toJson(
        Session(
          token: 'expired',
          expiresAt: DateTime.now().subtract(const Duration(seconds: 1)),
        ),
      ),
    );
    final expired = SessionController(
      storage,
      apiClient: ApiClient(Dio()),
    );
    await expired.initialize();
    expect(expired.loggedIn, isFalse);
    expect(storage.value, isNull);
    expired.dispose();
  });

  test('activation persists session; logout revokes and clears it', () async {
    final storage = MemoryStorage();
    final adapter = AuthAdapter();
    final controller = SessionController(
      storage,
      apiClient: ApiClient(Dio()..httpClientAdapter = adapter),
    );
    await controller.initialize();
    await controller.activate(
      Session(
        token: 'token-a',
        expiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    expect(controller.loggedIn, isTrue);
    expect(
      (jsonDecode(storage.value!) as Map).keys,
      unorderedEquals(['access_token', 'expires_at']),
    );
    final generation = controller.generation;
    await controller.logout();
    expect(adapter.authorization, 'Bearer token-a');
    expect(controller.loggedIn, isFalse);
    expect(controller.generation, greaterThan(generation));
    expect(storage.value, isNull);
    controller.dispose();
  });

  test('stale 401 cannot remove a newer login', () async {
    final storage = MemoryStorage();
    final adapter = AuthAdapter();
    final controller = SessionController(
      storage,
      apiClient: ApiClient(Dio()..httpClientAdapter = adapter),
    );
    await controller.initialize();
    await controller.activate(
      Session(
        token: 'token-a',
        expiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    await controller.clearIfCurrent('token-a');
    await controller.activate(
      Session(
        token: 'token-b',
        expiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    await controller.clearIfCurrent('token-a');
    expect(controller.token, 'token-b');
    expect(storage.value, contains('token-b'));
    controller.dispose();
  });

  test(
    'network failure during logout keeps session available for retry',
    () async {
      final storage = MemoryStorage();
      final adapter = AuthAdapter()..logoutStatus = 503;
      final controller = SessionController(
        storage,
        apiClient: ApiClient(Dio()..httpClientAdapter = adapter),
      );
      await controller.initialize();
      await controller.activate(
        Session(
          token: 'token-a',
          expiresAt: DateTime.now().add(const Duration(hours: 1)),
        ),
      );
      await expectLater(controller.logout(), throwsA(isA<Exception>()));
      expect(controller.loggedIn, isTrue);
      adapter.logoutStatus = 401;
      await controller.logout();
      expect(controller.loggedIn, isFalse);
      controller.dispose();
    },
  );

  test('revoking an unpublished token preserves the active session', () async {
    final storage = MemoryStorage();
    final adapter = AuthAdapter()..logoutStatus = 401;
    final controller = SessionController(
      storage,
      apiClient: ApiClient(Dio()..httpClientAdapter = adapter),
    );
    addTearDown(controller.dispose);
    await controller.activate(
      Session(
        token: 'current',
        expiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    await controller.revoke(
      Session(
        token: 'unpublished',
        expiresAt: DateTime.now().add(const Duration(hours: 1)),
      ),
    );
    expect(adapter.authorization, 'Bearer unpublished');
    expect(controller.token, 'current');
    expect(storage.value, contains('current'));
  });

  test('secure storage failure revokes newly issued token', () async {
    final storage = MemoryStorage()..failWrite = true;
    final adapter = AuthAdapter();
    final controller = SessionController(
      storage,
      apiClient: ApiClient(Dio()..httpClientAdapter = adapter),
    );
    await controller.initialize();
    await expectLater(
      controller.activate(
        Session(
          token: 'token-a',
          expiresAt: DateTime.now().add(const Duration(hours: 1)),
        ),
      ),
      throwsA(
        isA<ApiException>().having(
          (e) => e.message,
          "message",
          contains("端末に認証情報を保存できません"),
        ),
      ),
    );
    expect(controller.loggedIn, isFalse);
    expect(adapter.logoutCount, 1);
    controller.dispose();
  });
}
