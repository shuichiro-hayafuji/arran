import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/core/network/api_client.dart';
import 'package:spendable_today/features/login/repository/login_repository.dart';
import 'package:spendable_today/features/login/screens/login/login_intent.dart';
import 'package:spendable_today/features/login/screens/login/login_screen.dart';
import 'package:spendable_today/features/login/screens/login/login_state.dart';
import 'package:spendable_today/features/login/screens/login/login_view_model.dart';
import 'package:spendable_today/features/session/domain/session.dart';
import 'package:spendable_today/features/session/provider/session_provider.dart';

import 'session_test.dart' show MemoryStorage, AuthAdapter;

class TestLoginViewModel extends LoginViewModel {
  TestLoginViewModel(super.repository, super.session);
  LoginState get snapshot => state;
}

class FakeLoginRepository implements LoginRepository {
  int calls = 0;
  final result = Completer<Session>();
  @override
  Future<Session> login(String username, String password) {
    calls++;
    return result.future;
  }
}

Session liveSession() => Session(
  token: 'login-token',
  expiresAt: DateTime.now().add(const Duration(hours: 1)),
);

class LoginAdapter implements HttpClientAdapter {
  RequestOptions? request;
  int status = 200;
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? stream,
    Future<void>? cancelFuture,
  ) async {
    request = options;
    return ResponseBody.fromString(
      jsonEncode({
        'access_token': 'login-token',
        'expires_at': liveSession().expiresAt.toIso8601String(),
      }),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test(
    'repository uses the public ApiClient and preserves login error messages',
    () async {
      final adapter = LoginAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      addTearDown(dio.close);
      final container = ProviderContainer(
        overrides: [
          publicApiClientProvider.overrideWithValue(
            ApiClient(dio, needAuthorize: false),
          ),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(loginRepositoryProvider, (_, _) {});
      addTearDown(subscription.close);
      final repository = container.read(loginRepositoryProvider);
      final session = await repository.login(' alice ', ' private password ');
      expect(adapter.request!.path, '/auth/login');
      expect(adapter.request!.data, {
        'username': 'alice',
        'password': ' private password ',
      });
      expect(adapter.request!.headers.containsKey('Authorization'), isFalse);
      expect(session.token, 'login-token');
      expect(session.alive, isTrue);
      for (final entry in {
        401: '通信に失敗しました。再試行してください。',
        404: '接続先にログインAPIがありません。',
        503: 'サーバーの認証処理に失敗しました。',
      }.entries) {
        final status = entry.key;
        adapter.status = status;
        await expectLater(
          repository.login('alice', 'password'),
          throwsA(
            isA<ApiException>()
                .having((error) => error.statusCode, 'status', status)
                .having(
                  (error) => error.message,
                  'message',
                  contains(entry.value),
                ),
          ),
        );
      }
    },
  );

  test(
    'intent validates, blocks duplicate submits, and activates the session',
    () async {
      final storage = MemoryStorage();
      final controller = SessionController(storage);
      final repository = FakeLoginRepository();
      final vm = TestLoginViewModel(repository, controller);
      addTearDown(vm.dispose);
      addTearDown(controller.dispose);
      await vm.dispatch(const SubmitLogin(' ', 'password'));
      expect(repository.calls, 0);
      expect(vm.snapshot.error, isNotNull);
      final pending = vm.dispatch(const SubmitLogin('alice', 'password'));
      expect(vm.snapshot.busy, isTrue);
      await vm.dispatch(const SubmitLogin('alice', 'password'));
      expect(repository.calls, 1);
      repository.result.complete(liveSession());
      await pending;
      expect(controller.token, 'login-token');
      expect(controller.generation, 1);
      expect(storage.value, isNot(contains('password')));
      expect(vm.snapshot.busy, isFalse);
    },
  );

  test(
    'authentication failure leaves the session unchanged and allows retry',
    () async {
      final controller = SessionController(MemoryStorage());
      final repository = FakeLoginRepository();
      final vm = TestLoginViewModel(repository, controller);
      addTearDown(vm.dispose);
      addTearDown(controller.dispose);
      final pending = vm.dispatch(const SubmitLogin('alice', 'wrong'));
      repository.result.completeError(
        const ApiException('認証に失敗しました。', statusCode: 401),
      );
      await pending;
      expect(vm.snapshot.error, '認証に失敗しました。');
      expect(vm.snapshot.busy, isFalse);
      expect(controller.loggedIn, isFalse);
      await vm.dispatch(const SubmitLogin('alice', 'retry'));
      expect(repository.calls, 2);
    },
  );

  for (final disposed in [true, false]) {
    test(
      'late response is revoked after ${disposed ? "disposal" : "session change"}',
      () async {
        final adapter = AuthAdapter();
        final storage = MemoryStorage();
        final controller = SessionController(
          storage,
          dio: Dio()..httpClientAdapter = adapter,
        );
        final repository = FakeLoginRepository();
        final vm = TestLoginViewModel(repository, controller);
        addTearDown(controller.dispose);
        final pending = vm.dispatch(const SubmitLogin('alice', 'password'));
        if (disposed) {
          vm.dispose();
        } else {
          await controller.activate(
            Session(token: 'newer', expiresAt: liveSession().expiresAt),
          );
          addTearDown(vm.dispose);
        }
        repository.result.complete(liveSession());
        await pending;
        expect(controller.token, disposed ? isNull : 'newer');
        expect(adapter.authorization, 'Bearer login-token');
        expect(adapter.logoutCount, 1);
      },
    );
  }

  testWidgets(
    'login screen submits intent, disables controls, and displays failure',
    (tester) async {
      final controller = SessionController(MemoryStorage());
      await controller.initialize();
      final repository = FakeLoginRepository();
      addTearDown(controller.dispose);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sessionControllerProvider.overrideWithValue(controller),
            loginRepositoryProvider.overrideWithValue(repository),
          ],
          child: const MaterialApp(home: LoginScreen()),
        ),
      );
      await tester.enterText(find.byType(TextField).at(0), 'alice');
      await tester.enterText(find.byType(TextField).at(1), 'password');
      await tester.tap(find.byType(FilledButton));
      await tester.pump();
      expect(find.text('ログイン中…'), findsOneWidget);
      expect(
        tester.widget<FilledButton>(find.byType(FilledButton)).onPressed,
        isNull,
      );
      repository.result.completeError(const ApiException('認証に失敗しました。'));
      await tester.pumpAndSettle();
      expect(find.text('認証に失敗しました。'), findsOneWidget);
      expect(find.text('alice'), findsOneWidget);
      expect(
        tester.widget<TextField>(find.byType(TextField).at(1)).controller!.text,
        isEmpty,
      );
      await tester.pumpWidget(const SizedBox());
    },
  );
}
