import 'dart:convert';
import 'dart:io';
import 'dart:async';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/core/network/api_client.dart';
import 'package:spendable_today/features/dashboard/screens/dashboard/dashboard_view_model.dart';
import 'package:spendable_today/features/dashboard/repository/dashboard_repository.dart';
import 'package:spendable_today/features/transactions/domain/transaction_item.dart';
import 'package:spendable_today/features/session/domain/session.dart';
import 'package:spendable_today/features/session/provider/session_provider.dart';
import 'package:spendable_today/core/network/interceptor/request_interceptor.dart';
import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/features/transactions/domain/import_commit_result.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/repository/pending_import_storage.dart';
import 'package:spendable_today/features/transactions/repository/transactions_repository.dart';
import 'package:spendable_today/features/transactions/screens/import/import_intent.dart';
import 'package:spendable_today/features/transactions/screens/import/import_screen.dart';
import 'package:spendable_today/features/transactions/screens/import/import_view_model.dart';

import 'widget_test.dart' show FakeTransactionsRepository;

class UncertainRepository extends FakeTransactionsRepository {
  final ids = <String>[];
  final failures = <Object>[];
  int previews = 0;

  @override
  Future<ImportCommitResult> commitImport(
    String previewId, {
    required int sessionGeneration,
  }) async {
    ids.add(previewId);
    if (failures.isNotEmpty) throw failures.removeAt(0);
    return const ImportCommitResult(importedCount: 2, duplicateCount: 0);
  }

  @override
  Future<ImportPreview> previewImport({
    int? sessionGeneration,
    required String filePath,
    required String fileName,
    required String source,
    required String accountName,
    bool confirmReactivation = false,
    CsvMapping? mapping,
    List<ImportIdentityDecision> identityDecisions = const [],
    List<PurchaseCsvDecision> reconciliationDecisions = const [],
  }) {
    previews++;
    return super.previewImport(
      sessionGeneration: sessionGeneration,
      filePath: filePath,
      fileName: fileName,
      source: source,
      accountName: accountName,
      confirmReactivation: confirmReactivation,
      mapping: mapping,
      identityDecisions: identityDecisions,
      reconciliationDecisions: reconciliationDecisions,
    );
  }
}

class FailingRefreshRepository extends UncertainRepository {
  FailingRefreshRepository(this.real);
  final TransactionsRepositoryImpl real;
  @override
  Future<List<TransactionItem>> getAll() => real.getAll();
}

class LogoutRefreshRepository extends UncertainRepository {
  LogoutRefreshRepository(this.session);
  final SessionController session;
  @override
  Future<List<TransactionItem>> getAll() async {
    await session.clearIfCurrent('token-a');
    throw const ApiException('unauthorized', statusCode: 401);
  }
}

class CommitAdapter implements HttpClientAdapter {
  int status = 200;
  Object? body;
  int calls = 0;
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? stream,
    Future<void>? cancelFuture,
  ) async {
    calls++;
    return ResponseBody.fromString(
      jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

class TestSessionStorage implements SessionStorage {
  String? value;
  @override
  Future<String?> read() async => value;
  @override
  Future<void> write(String value) async {
    this.value = value;
  }

  @override
  Future<void> delete() async {
    value = null;
  }
}

class DelayedOwnerAdapter extends CommitAdapter {
  final started = Completer<void>();
  final resume = Completer<void>();
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? stream,
    Future<void>? cancelFuture,
  ) async {
    started.complete();
    await resume.future;
    return super.fetch(options, stream, cancelFuture);
  }
}

class DelayedCommitRepository extends UncertainRepository {
  final started = Completer<void>();
  final result = Completer<ImportCommitResult>();
  @override
  Future<ImportCommitResult> commitImport(
    String previewId, {
    required int sessionGeneration,
  }) {
    ids.add(previewId);
    started.complete();
    return result.future;
  }
}

class TestVault implements PendingImportVault {
  final values = <int, String>{};
  @override
  Future<String?> read(int ownerId) async => values[ownerId];
  @override
  Future<void> write(int ownerId, String value) async {
    values[ownerId] = value;
  }

  @override
  Future<void> delete(int ownerId) async {
    values.remove(ownerId);
  }
}

class BlockingVault extends TestVault {
  final started = Completer<void>();
  final resume = Completer<void>();
  bool block = true;
  @override
  Future<void> write(int ownerId, String value) async {
    if (block) {
      block = false;
      started.complete();
      await resume.future;
    }
    await super.write(ownerId, value);
  }
}

class FailingStorage implements PendingImportStorage {
  FailingStorage({this.failLoad = false});
  final bool failLoad;
  @override
  Future<String?> load(int? scope) async {
    if (failLoad) throw StateError('load failed');
    return null;
  }

  @override
  Future<void> save(int? scope, String previewId) async =>
      throw StateError('save failed');
  @override
  Future<void> clear(int? scope, String previewId) async {}
}

class ClearOnceStorage extends MemoryPendingImportStorage {
  bool fail = true;
  @override
  Future<void> clear(int? scope, String previewId) async {
    if (fail) {
      fail = false;
      throw StateError('clear failed');
    }
    await super.clear(scope, previewId);
  }
}

class DelayedPreviewRepository extends UncertainRepository {
  final started = Completer<void>();
  final resume = Completer<void>();
  int requests = 0;
  @override
  Future<ImportPreview> previewImport({
    int? sessionGeneration,
    required String filePath,
    required String fileName,
    required String source,
    required String accountName,
    bool confirmReactivation = false,
    CsvMapping? mapping,
    List<ImportIdentityDecision> identityDecisions = const [],
    List<PurchaseCsvDecision> reconciliationDecisions = const [],
  }) async {
    requests++;
    if (requests == 2) {
      started.complete();
      await resume.future;
    }
    return super.previewImport(
      sessionGeneration: sessionGeneration,
      filePath: filePath,
      fileName: fileName,
      source: source,
      accountName: accountName,
      confirmReactivation: confirmReactivation,
      mapping: mapping,
      identityDecisions: identityDecisions,
      reconciliationDecisions: reconciliationDecisions,
    );
  }
}

SessionController testSession() =>
    SessionController(TestSessionStorage(), apiClient: ApiClient(Dio()));

Future<void> login(SessionController session, String token) => session.activate(
  Session(
    token: token,
    expiresAt: DateTime.now().add(const Duration(hours: 1)),
  ),
);

void main() {
  test(
    'commit requires a complete 200 receipt, including zero results',
    () async {
      final adapter = CommitAdapter();
      final dio = Dio(
        BaseOptions(
          baseUrl: 'https://example.invalid',
          followRedirects: false,
          validateStatus: (status) => status != null && status < 400,
        ),
      )..httpClientAdapter = adapter;
      final repository = TransactionsRepositoryImpl(ApiClient(dio));
      const receipt = {
        'import_id': '',
        'imported_count': 0,
        'duplicate_count': 0,
        'reactivated_count': 0,
        'financial_pending': 0,
      };
      for (final code in [301, 307]) {
        adapter.status = code;
        adapter.body = receipt;
        await expectLater(
          repository.commitImport('id', sessionGeneration: 0),
          throwsA(isA<ApiException>()),
        );
      }
      adapter.status = 200;
      for (final bad in [
        null,
        {},
        {'imported_count': 0},
        {...receipt, 'duplicate_count': '0'},
        {...receipt, 'imported_count': 1},
        {...receipt, 'duplicate_count': 1},
        {...receipt, 'reactivated_count': 1},
        {...receipt, 'financial_pending': 1},
      ]) {
        adapter.body = bad;
        await expectLater(
          repository.commitImport('id', sessionGeneration: 0),
          throwsA(isA<ApiException>()),
        );
      }
      adapter.body = receipt;
      final result = await repository.commitImport('id', sessionGeneration: 0);
      expect(result.importedCount, 0);
      expect(result.duplicateCount, 0);
    },
  );

  test(
    'unknown, then 404 keeps the same ID and prevents changing input',
    () async {
      final repository = UncertainRepository()
        ..failures.addAll([
          const ApiException('timeout'),
          const ApiException('not found', statusCode: 404),
        ]);
      final container = ProviderContainer(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await vm.commit();
      expect(
        container.read(importControllerProvider).commitOutcomeUnknown,
        isTrue,
      );
      await vm.selectAndPreview(filePath: '/tmp/b.csv', fileName: 'b.csv');
      await vm.remap(const CsvMapping(dateColumn: 'other'));
      await vm.setSource('other', 'other');
      await vm.dispatch(const ConfirmImportReactivation());
      await vm.resolve(const []);
      await vm.resolvePurchases(const []);
      await vm.reset();
      expect(repository.previews, 1);
      expect(
        container.read(importControllerProvider).preview!.previewId,
        'preview-1',
      );
      await vm.commit();
      expect(
        container.read(importControllerProvider).commitOutcomeUnknown,
        isTrue,
      );
      expect(repository.ids, ['preview-1', 'preview-1']);
      await vm.commit();
      expect(repository.ids, ['preview-1', 'preview-1', 'preview-1']);
      expect(
        container.read(importControllerProvider).commitOutcomeUnknown,
        isFalse,
      );
      expect(
        container.read(importControllerProvider).commitResult!.importedCount,
        2,
      );
      await vm.commit();
      expect(repository.ids.length, 3);
      await vm.reset();
      expect(container.read(importControllerProvider).preview, isNull);
    },
  );

  test(
    'scope isolation and conditional clear never expose another session',
    () async {
      final vault = TestVault();
      var generation = 1;
      final first = SecurePendingImportStorage(
        () => generation == 1,
        vault: vault,
      );
      await first.save(101, 'preview-a');
      expect(await first.load(101), 'preview-a');
      generation = 2;
      final second = SecurePendingImportStorage(
        () => generation == 2,
        vault: vault,
      );
      expect(await second.load(202), isNull);
      await expectLater(first.save(101, 'stale'), throwsStateError);
      await second.save(202, 'preview-b');
      await expectLater(second.save(202, 'different'), throwsStateError);
      await expectLater(first.clear(101, 'preview-a'), throwsStateError);
      await expectLater(second.clear(202, 'preview-a'), throwsStateError);
      expect(await second.load(202), 'preview-b');
      await expectLater(second.load(101), throwsStateError);
      generation = 3;
      final relogin = SecurePendingImportStorage(
        () => generation == 3,
        vault: vault,
      );
      expect(await relogin.load(101), 'preview-a');
    },
  );

  test(
    'late prior-session write cannot replace new-session pending ID',
    () async {
      final vault = BlockingVault();
      var generation = 1;
      final first = SecurePendingImportStorage(
        () => generation == 1,
        vault: vault,
      );
      final oldWrite = first.save(101, 'old');
      final oldResult = expectLater(oldWrite, throwsStateError);
      await vault.started.future;
      generation = 2;
      final second = SecurePendingImportStorage(
        () => generation == 2,
        vault: vault,
      );
      final newWrite = second.save(202, 'new');
      vault.resume.complete();
      await oldResult;
      await newWrite;
      expect(await second.load(202), 'new');
      await expectLater(second.load(101), throwsStateError);
      generation = 3;
      final relogin = SecurePendingImportStorage(
        () => generation == 3,
        vault: vault,
      );
      expect(await relogin.load(101), 'old');
    },
  );

  test(
    'owner lookup rejects non-200, malformed ID, and changed session',
    () async {
      final session = testSession();
      addTearDown(session.dispose);
      await login(session, 'token-a');
      final adapter = CommitAdapter()..body = {'id': 101};
      final dio = Dio(
        BaseOptions(
          baseUrl: 'https://example.invalid',
          validateStatus: (status) => status != null && status < 400,
        ),
      )..httpClientAdapter = adapter;
      final container = ProviderContainer(
        overrides: [
          sessionControllerProvider.overrideWithValue(session),
          authorizedApiClientProvider.overrideWithValue(ApiClient(dio)),
        ],
      );
      addTearDown(container.dispose);
      final resolve = container.read(pendingImportOwnerResolverProvider);
      expect(await resolve(), 101);
      adapter.status = 302;
      await expectLater(resolve(), throwsFormatException);
      adapter.status = 200;
      adapter.body = {'id': '101'};
      await expectLater(resolve(), throwsFormatException);
      adapter.body = {'id': 0};
      await expectLater(resolve(), throwsFormatException);
      await login(session, 'token-b');
      await expectLater(resolve(), throwsStateError);
    },
  );

  test(
    'delayed owner response after logout cannot identify a new session',
    () async {
      final session = testSession();
      addTearDown(session.dispose);
      await login(session, 'token-a');
      final adapter = DelayedOwnerAdapter()..body = {'id': 101};
      final dio = Dio(BaseOptions(baseUrl: 'https://example.invalid'))
        ..httpClientAdapter = adapter;
      final container = ProviderContainer(
        overrides: [
          sessionControllerProvider.overrideWithValue(session),
          authorizedApiClientProvider.overrideWithValue(ApiClient(dio)),
        ],
      );
      addTearDown(container.dispose);
      final pending = container.read(pendingImportOwnerResolverProvider)();
      final failure = expectLater(pending, throwsStateError);
      await adapter.started.future;
      await session.clearIfCurrent('token-a');
      await login(session, 'token-b');
      adapter.resume.complete();
      await failure;
    },
  );

  test(
    'authentication failure during owner lookup blocks preview and commit',
    () async {
      final session = testSession();
      addTearDown(session.dispose);
      await login(session, 'token-a');
      final adapter = CommitAdapter()
        ..status = 401
        ..body = {'message': 'unauthorized'};
      final dio = Dio(BaseOptions(baseUrl: 'https://example.invalid'))
        ..httpClientAdapter = adapter;
      final repository = UncertainRepository();
      final token = session.token;
      final generation = session.generation;
      final container = ProviderContainer(
        overrides: [
          sessionControllerProvider.overrideWithValue(session),
          authorizedApiClientProvider.overrideWithValue(ApiClient(dio)),
          transactionsRepositoryProvider.overrideWithValue(repository),
          pendingImportStorageProvider.overrideWithValue(
            SecurePendingImportStorage(
              () => session.token == token && session.generation == generation,
              vault: TestVault(),
            ),
          ),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await vm.commit();
      expect(container.read(importControllerProvider).recoveryBlocked, isTrue);
      expect(repository.previews, 0);
      expect(repository.ids, isEmpty);
    },
  );

  test(
    'missing session blocks preview and commit before calling repository',
    () async {
      final session = testSession();
      addTearDown(session.dispose);
      final repository = UncertainRepository();
      final container = ProviderContainer(
        overrides: [
          sessionControllerProvider.overrideWithValue(session),
          transactionsRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await vm.commit();
      expect(container.read(importControllerProvider).recoveryBlocked, isTrue);
      expect(repository.previews, 0);
      expect(repository.ids, isEmpty);
    },
  );

  test(
    'session switch before HTTP interceptor prevents cross-user submission',
    () async {
      final session = testSession();
      addTearDown(session.dispose);
      await login(session, 'token-a');
      final container = ProviderContainer(
        overrides: [sessionControllerProvider.overrideWithValue(session)],
      );
      addTearDown(container.dispose);
      final ref = container.read(Provider<Ref>((ref) => ref));
      final started = Completer<void>();
      final resume = Completer<void>();
      final adapter = CommitAdapter();
      final dio = Dio(
        BaseOptions(
          baseUrl: 'https://example.invalid',
          validateStatus: (status) => status != null && status < 400,
        ),
      );
      dio.httpClientAdapter = adapter;
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (options, handler) async {
            started.complete();
            await resume.future;
            handler.next(options);
          },
        ),
      );
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (options, handler) {
            requestInterceptor(options, handler, true, ref);
          },
        ),
      );
      final pending = ApiClient(dio).post(
        '/transactions/import/commit',
        options: Options(
          extra: {expectedSessionGenerationKey: session.generation},
        ),
      );
      final rejected = expectLater(pending, throwsA(isA<ApiException>()));
      await started.future;
      await login(session, 'token-b');
      resume.complete();
      await rejected;
      expect(adapter.calls, 0);
    },
  );

  test(
    'session switch before preview interceptor prevents cross-user CSV submission',
    () async {
      final session = testSession();
      addTearDown(session.dispose);
      await login(session, 'token-a');
      final container = ProviderContainer(
        overrides: [sessionControllerProvider.overrideWithValue(session)],
      );
      addTearDown(container.dispose);
      final ref = container.read(Provider<Ref>((ref) => ref));
      final started = Completer<void>();
      final resume = Completer<void>();
      final adapter = CommitAdapter();
      final dio = Dio(
        BaseOptions(
          baseUrl: 'https://example.invalid',
          validateStatus: (status) => status != null && status < 400,
        ),
      );
      dio.httpClientAdapter = adapter;
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (options, handler) async {
            started.complete();
            await resume.future;
            handler.next(options);
          },
        ),
      );
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (options, handler) {
            requestInterceptor(options, handler, true, ref);
          },
        ),
      );
      final directory = await Directory.systemTemp.createTemp(
        'import-preview-test',
      );
      addTearDown(() => directory.delete(recursive: true));
      final file = File('${directory.path}/test.csv');
      await file.writeAsString('date,amount\n2026-01-01,100');
      final pending = TransactionsRepositoryImpl(ApiClient(dio)).previewImport(
        filePath: file.path,
        fileName: 'test.csv',
        source: 'bank',
        accountName: 'account-a',
        sessionGeneration: session.generation,
      );
      final rejected = expectLater(pending, throwsA(isA<ApiException>()));
      await started.future;
      await login(session, 'token-b');
      resume.complete();
      await rejected;
      expect(adapter.calls, 0);
    },
  );

  test(
    'same user can recover after login; other user cannot see or overwrite',
    () async {
      final session = testSession();
      addTearDown(session.dispose);
      final vault = TestVault();
      final repository = UncertainRepository()
        ..failures.add(const ApiException('timeout'));
      Future<ProviderContainer> open(int ownerId) async {
        final token = session.token;
        final generation = session.generation;
        final container = ProviderContainer(
          overrides: [
            sessionControllerProvider.overrideWithValue(session),
            transactionsRepositoryProvider.overrideWithValue(repository),
            pendingImportOwnerResolverProvider.overrideWithValue(
              () async => ownerId,
            ),
            pendingImportStorageProvider.overrideWithValue(
              SecurePendingImportStorage(
                () =>
                    session.token == token && session.generation == generation,
                vault: vault,
              ),
            ),
          ],
        );
        addTearDown(container.dispose);
        return container;
      }

      await login(session, 'token-a');
      final a = await open(101);
      final aVm = a.read(importControllerProvider.notifier);
      await aVm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await aVm.commit();
      expect(a.read(importControllerProvider).commitOutcomeUnknown, isTrue);
      await session.clearIfCurrent('token-a');
      await login(session, 'token-b');
      final b = await open(202);
      final bVm = b.read(importControllerProvider.notifier);
      await bVm.selectAndPreview(filePath: '/tmp/b.csv', fileName: 'b.csv');
      expect(b.read(importControllerProvider).commitOutcomeUnknown, isFalse);
      expect(b.read(importControllerProvider).preview!.previewId, 'preview-1');
      await bVm.commit();
      expect(vault.values.keys, containsAll([101, 202]));
      await session.clearIfCurrent('token-b');
      await login(session, 'token-a-again');
      final aAgain = await open(101);
      await aAgain.read(importControllerProvider.notifier).commit();
      expect(repository.ids, ['preview-1', 'preview-1', 'preview-1']);
      expect(aAgain.read(importControllerProvider).commitResult, isNotNull);
    },
  );

  test(
    'late commit response does not cross sessions or discard pending ID',
    () async {
      final session = testSession();
      addTearDown(session.dispose);
      await login(session, 'token-a');
      final token = session.token;
      final generation = session.generation;
      final vault = TestVault();
      final repository = DelayedCommitRepository();
      final container = ProviderContainer(
        overrides: [
          sessionControllerProvider.overrideWithValue(session),
          transactionsRepositoryProvider.overrideWithValue(repository),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
          pendingImportStorageProvider.overrideWithValue(
            SecurePendingImportStorage(
              () => session.token == token && session.generation == generation,
              vault: vault,
            ),
          ),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      final pending = vm.commit();
      await repository.started.future;
      await login(session, 'token-b');
      repository.result.complete(
        const ImportCommitResult(importedCount: 2, duplicateCount: 0),
      );
      await pending;
      expect(container.read(importControllerProvider).commitResult, isNull);
      expect(container.read(importControllerProvider).recoveryBlocked, isTrue);
      expect(vault.values[101], contains('preview-1'));
    },
  );

  test(
    'storage failure prevents sending and load failure prevents changes',
    () async {
      final repository = UncertainRepository();
      final container = ProviderContainer(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
          pendingImportStorageProvider.overrideWithValue(FailingStorage()),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await vm.commit();
      expect(repository.ids, isEmpty);
      expect(
        container.read(importControllerProvider).error,
        contains('取込は開始していません'),
      );
      final blocked = ProviderContainer(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
          pendingImportStorageProvider.overrideWithValue(
            FailingStorage(failLoad: true),
          ),
        ],
      );
      addTearDown(blocked.dispose);
      await blocked
          .read(importControllerProvider.notifier)
          .selectAndPreview(filePath: '/tmp/b.csv', fileName: 'b.csv');
      expect(blocked.read(importControllerProvider).recoveryBlocked, isTrue);
      expect(repository.previews, 1);
    },
  );

  test(
    'restart recovers an unacknowledged successful result using the same ID',
    () async {
      final repository = UncertainRepository();
      final storage = MemoryPendingImportStorage();
      ProviderContainer open() => ProviderContainer(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
          pendingImportStorageProvider.overrideWithValue(storage),
        ],
      );
      final first = open();
      final vm = first.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await vm.commit();
      expect(first.read(importControllerProvider).commitResult, isNotNull);
      expect(storage.value, 'preview-1');
      first.dispose();
      final restored = open();
      addTearDown(restored.dispose);
      final next = restored.read(importControllerProvider.notifier);
      await next.commit();
      expect(repository.ids, ['preview-1', 'preview-1']);
      expect(restored.read(importControllerProvider).commitResult, isNotNull);
      await next.reset();
      expect(storage.value, isNull);
    },
  );

  test('first definitive rejection keeps editing available', () async {
    final repository = UncertainRepository()
      ..failures.add(const ApiException('期限切れ', statusCode: 400));
    final container = ProviderContainer(
      overrides: [
        transactionsRepositoryProvider.overrideWithValue(repository),
        pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
      ],
    );
    addTearDown(container.dispose);
    final vm = container.read(importControllerProvider.notifier);
    await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
    await vm.commit();
    expect(
      container.read(importControllerProvider).commitOutcomeUnknown,
      isFalse,
    );
    expect(container.read(importControllerProvider).error, '期限切れ');
    await vm.selectAndPreview(filePath: '/tmp/b.csv', fileName: 'b.csv');
    expect(repository.previews, 2);
  });

  test(
    'rejection with failed local cleanup can retry cleanup without sending',
    () async {
      final repository = UncertainRepository()
        ..failures.add(const ApiException('期限切れ', statusCode: 400));
      final storage = ClearOnceStorage();
      final container = ProviderContainer(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
          pendingImportStorageProvider.overrideWithValue(storage),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await vm.commit();
      expect(container.read(importControllerProvider).cleanupRejected, isTrue);
      expect(
        container.read(importControllerProvider).commitOutcomeUnknown,
        isFalse,
      );
      await vm.commit();
      expect(repository.ids, ['preview-1']);
      await vm.retryRejectedCleanup();
      expect(container.read(importControllerProvider).cleanupRejected, isFalse);
      expect(storage.value, isNull);
      await vm.selectAndPreview(filePath: '/tmp/b.csv', fileName: 'b.csv');
      expect(repository.previews, 2);
    },
  );

  test(
    'operator recovery clears only the verified owner after restart',
    () async {
      final vault = TestVault();
      final session = testSession();
      addTearDown(session.dispose);
      await login(session, 'token-a');
      final storage = SecurePendingImportStorage(() => true, vault: vault);
      await storage.save(101, 'old-id');
      final other = SecurePendingImportStorage(() => true, vault: vault);
      await other.save(202, 'other-id');
      final container = ProviderContainer(
        overrides: [
          sessionControllerProvider.overrideWithValue(session),
          pendingImportStorageProvider.overrideWithValue(storage),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
          transactionsRepositoryProvider.overrideWithValue(
            UncertainRepository(),
          ),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      expect(
        container.read(importControllerProvider).pendingPreviewId,
        'old-id',
      );
      await vm.resumeAfterOperatorConfirmation();
      expect(await storage.load(101), isNull);
      expect(await other.load(202), 'other-id');
      expect(
        container.read(importControllerProvider).commitOutcomeUnknown,
        isFalse,
      );
    },
  );

  test(
    'real list and dashboard repository failures preserve receipt and warn',
    () async {
      final adapter = CommitAdapter()
        ..status = 500
        ..body = {};
      final dio = Dio(BaseOptions(baseUrl: 'https://example.invalid'))
        ..httpClientAdapter = adapter;
      final client = ApiClient(dio);
      final repository = FailingRefreshRepository(
        TransactionsRepositoryImpl(client),
      );
      final container = ProviderContainer(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
          dashboardRepositoryProvider.overrideWithValue(
            DashboardRepositoryImpl(client),
          ),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await vm.commit();
      await Future<void>.delayed(const Duration(milliseconds: 30));
      expect(container.read(importControllerProvider).commitResult, isNotNull);
      expect(
        container.read(importControllerProvider).commitOutcomeUnknown,
        isFalse,
      );
      expect(
        container.read(importControllerProvider).error,
        contains('保存は完了しました'),
      );
    },
  );

  test(
    'session loss during refresh hides the old receipt and preserves recovery',
    () async {
      final session = testSession();
      addTearDown(session.dispose);
      await login(session, 'token-a');
      final repository = LogoutRefreshRepository(session);
      final adapter = CommitAdapter()
        ..status = 500
        ..body = {};
      final dio = Dio(BaseOptions(baseUrl: 'https://example.invalid'))
        ..httpClientAdapter = adapter;
      final container = ProviderContainer(
        overrides: [
          sessionControllerProvider.overrideWithValue(session),
          transactionsRepositoryProvider.overrideWithValue(repository),
          dashboardRepositoryProvider.overrideWithValue(
            DashboardRepositoryImpl(ApiClient(dio)),
          ),
          pendingImportStorageProvider.overrideWithValue(
            MemoryPendingImportStorage(),
          ),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await vm.commit();
      await Future<void>.delayed(const Duration(milliseconds: 30));
      expect(container.read(importControllerProvider).recoveryBlocked, isTrue);
      expect(container.read(importControllerProvider).commitResult, isNull);
      expect(
        container.read(importControllerProvider).pendingPreviewId,
        'preview-1',
      );
    },
  );

  test('re-preview in progress prevents confirming the previous ID', () async {
    final repository = DelayedPreviewRepository();
    final container = ProviderContainer(
      overrides: [
        transactionsRepositoryProvider.overrideWithValue(repository),
        pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
      ],
    );
    addTearDown(container.dispose);
    final vm = container.read(importControllerProvider.notifier);
    await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
    final repreview = vm.dispatch(const ConfirmImportReactivation());
    await repository.started.future;
    expect(container.read(importControllerProvider).loading, isTrue);
    await vm.commit();
    expect(repository.ids, isEmpty);
    repository.resume.complete();
    await repreview;
    expect(container.read(importControllerProvider).loading, isFalse);
  });

  test(
    'refresh failure never changes a successful commit to unknown',
    () async {
      final repository = UncertainRepository();
      final container = ProviderContainer(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
          dashboardViewModelProvider.overrideWith(
            (ref) => throw StateError('failed'),
          ),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
      await vm.commit();
      expect(container.read(importControllerProvider).commitResult, isNotNull);
      expect(
        container.read(importControllerProvider).commitOutcomeUnknown,
        isFalse,
      );
      expect(
        container.read(importControllerProvider).error,
        contains('保存は完了しました'),
      );
    },
  );

  for (final disposeScreen in [false, true]) {
    testWidgets(
      'delayed file picker cannot submit after session switch or disposal: $disposeScreen',
      (tester) async {
        final session = testSession();
        await login(session, 'token-a');
        final picked = Completer<FilePickerResult?>();
        final repository = UncertainRepository();
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sessionControllerProvider.overrideWithValue(session),
              transactionsRepositoryProvider.overrideWithValue(repository),
              pendingImportOwnerResolverProvider.overrideWithValue(
                () async => 101,
              ),
              pendingImportStorageProvider.overrideWithValue(
                MemoryPendingImportStorage(),
              ),
              importFilePickerProvider.overrideWithValue(() => picked.future),
            ],
            child: const MaterialApp(home: ImportScreen()),
          ),
        );
        await tester.pumpAndSettle();
        await tester.ensureVisible(find.byKey(const ValueKey('pick-csv')));
        await tester.tap(find.byKey(const ValueKey('pick-csv')));
        await tester.pump();
        await session.clearIfCurrent('token-a');
        if (disposeScreen) {
          await tester.pumpWidget(const MaterialApp(home: SizedBox()));
        } else {
          await login(session, 'token-b');
        }
        picked.complete(
          FilePickerResult([
            PlatformFile(name: 'a.csv', size: 1, path: '/tmp/a.csv'),
          ]),
        );
        await tester.pumpAndSettle();
        expect(repository.previews, 0);
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const MaterialApp(home: SizedBox()));
        session.dispose();
      },
    );
  }

  testWidgets('unknown result prevents editing and warns before leaving', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final repository = UncertainRepository()
      ..failures.add(const ApiException('timeout'));
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
          pendingImportOwnerResolverProvider.overrideWithValue(() async => 101),
        ],
        child: MaterialApp(
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                onPressed: () => Navigator.of(context).push(
                  MaterialPageRoute<void>(builder: (_) => const ImportScreen()),
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    final container = ProviderScope.containerOf(
      tester.element(find.byType(ImportScreen)),
    );
    await container
        .read(importControllerProvider.notifier)
        .selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
    await container.read(importControllerProvider.notifier).commit();
    await tester.pumpAndSettle();
    expect(find.textContaining('保存結果が不明です'), findsOneWidget);
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('import-source')))
          .enabled,
      isFalse,
    );
    expect(
      tester
          .widget<OutlinedButton>(find.byKey(const ValueKey('pick-csv')))
          .onPressed,
      isNull,
    );
    await tester.ensureVisible(find.text('運用者に確認した場合の再開'));
    await tester.tap(find.text('運用者に確認した場合の再開'));
    await tester.pumpAndSettle();
    final dialogInput = find.descendant(
      of: find.byType(AlertDialog),
      matching: find.byType(TextField),
    );
    final resume = find.widgetWithText(TextButton, '確認情報を削除して再開');
    expect(tester.widget<TextButton>(resume).onPressed, isNull);
    await tester.enterText(dialogInput, '未保存');
    await tester.pumpAndSettle();
    expect(tester.widget<TextButton>(resume).onPressed, isNull);
    await tester.tap(find.text('戻る'));
    await tester.pumpAndSettle();
    expect(
      container.read(importControllerProvider).commitOutcomeUnknown,
      isTrue,
    );
    expect(
      container.read(importControllerProvider).pendingPreviewId,
      'preview-1',
    );
    await tester.tap(find.text('運用者に確認した場合の再開'));
    await tester.pumpAndSettle();
    await tester.enterText(dialogInput, '未保存を確認しました');
    await tester.pumpAndSettle();
    expect(tester.widget<TextButton>(resume).onPressed, isNotNull);
    await tester.tap(resume);
    await tester.pumpAndSettle();
    expect(
      container.read(importControllerProvider).commitOutcomeUnknown,
      isFalse,
    );
    expect(container.read(importControllerProvider).pendingPreviewId, isNull);
    await container
        .read(importControllerProvider.notifier)
        .selectAndPreview(filePath: '/tmp/a.csv', fileName: 'a.csv');
    repository.failures.add(const ApiException('timeout'));
    await container.read(importControllerProvider.notifier).commit();
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(const ValueKey('commit-import')));
    expect(find.text('保存結果を確認'), findsOneWidget);
    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(find.text('保存結果を確認できていません'), findsOneWidget);
    expect(find.textContaining('同じ利用者で再ログイン'), findsWidgets);
    await tester.tap(find.text('確認を続ける'));
    await tester.pumpAndSettle();
    expect(find.byType(ImportScreen), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
