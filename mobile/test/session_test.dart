import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:spendable_today/core/network/api_exception.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/session/domain/session.dart';
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
  String issuedToken = 'token-a';
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
    return ResponseBody.fromString(
      jsonEncode({
        'access_token': issuedToken,
        'expires_at': DateTime.now()
            .add(const Duration(hours: 1))
            .toIso8601String(),
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test('restores live sessions and removes expired sessions', () async {
    final storage = MemoryStorage();
    storage.value = jsonEncode(
      Session(
        token: 'saved',
        expiresAt: DateTime.now().add(const Duration(hours: 1)),
      ).toJson(),
    );
    final controller = SessionController(storage);
    await controller.initialize();
    expect(controller.token, 'saved');
    controller.dispose();
    storage.value = jsonEncode(
      Session(
        token: 'expired',
        expiresAt: DateTime.now().subtract(const Duration(seconds: 1)),
      ).toJson(),
    );
    final expired = SessionController(storage);
    await expired.initialize();
    expect(expired.loggedIn, isFalse);
    expect(storage.value, isNull);
    expired.dispose();
  });

  test('login persists token only; logout revokes and clears it', () async {
    final storage = MemoryStorage();
    final adapter = AuthAdapter();
    final controller = SessionController(
      storage,
      dio: Dio()..httpClientAdapter = adapter,
    );
    await controller.initialize();
    await controller.login('alice', 'private password');
    expect(controller.loggedIn, isTrue);
    expect(storage.value, isNot(contains('private password')));
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
      dio: Dio()..httpClientAdapter = adapter,
    );
    await controller.initialize();
    await controller.login('alice', 'password');
    await controller.clearIfCurrent('token-a');
    adapter.issuedToken = 'token-b';
    await controller.login('bob', 'password');
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
        dio: Dio()..httpClientAdapter = adapter,
      );
      await controller.initialize();
      await controller.login('alice', 'password');
      await expectLater(controller.logout(), throwsA(isA<Exception>()));
      expect(controller.loggedIn, isTrue);
      adapter.logoutStatus = 401;
      await controller.logout();
      expect(controller.loggedIn, isFalse);
      controller.dispose();
    },
  );

  test('secure storage failure revokes newly issued token', () async {
    final storage = MemoryStorage()..failWrite = true;
    final adapter = AuthAdapter();
    final controller = SessionController(
      storage,
      dio: Dio()..httpClientAdapter = adapter,
    );
    await controller.initialize();
    await expectLater(
      controller.login('alice', 'password'),
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
