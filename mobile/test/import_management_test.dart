import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/core/network/api_exception.dart';
import 'package:spendable_today/features/dashboard/repository/dashboard_repository.dart';
import 'package:spendable_today/features/transactions/domain/import_management.dart';
import 'package:spendable_today/features/transactions/domain/transaction_item.dart';
import 'package:spendable_today/features/transactions/repository/import_management_repository.dart';
import 'package:spendable_today/features/transactions/repository/transactions_repository.dart';
import 'package:spendable_today/features/transactions/repository/dto/import_management_dto.dart';
import 'package:spendable_today/features/transactions/screens/history/management_intent.dart';
import 'package:spendable_today/features/transactions/screens/history/management_screen.dart';
import 'package:spendable_today/features/transactions/screens/history/import_history_screen.dart';
import 'package:spendable_today/features/transactions/screens/history/management_view_model.dart';
import 'widget_test.dart' show FakeRepositories, FakeTransactionsRepository;
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/features/transactions/screens/import/import_view_model.dart';
import 'package:spendable_today/features/transactions/screens/import/import_intent.dart';

const item = TransactionItem(
  id: 1,
  transactionDate: '2035-01-01',
  description: 'OWNER BANK',
  normalizedMerchant: 'OWNER BANK',
  amount: 2500,
  transactionType: 'expense',
  category: '未分類',
  source: 'bank',
  sourceAccountName: 'account',
);
ImportBatch fixtureBatch({String status = 'active'}) => ImportBatch(
  id: 'batch',
  fileName: 'bank.csv',
  source: 'bank',
  account: 'account',
  status: status,
  version: 1,
  stateVersion: 7,
  snapshot: 'same-snapshot',
  recordedAt: '',
  legacy: true,
  affected: 1,
  protected: 1,
  rows: [const ImportEvidence(2, 1, item, item, true, false)],
);

class ManagementFake implements ImportManagementRepository {
  final requests = <MutationRequest>[];
  MutationResult? saved;
  Object? reject;
  Completer<MutationResult>? delayed;
  bool loseResponse = false;
  @override
  Future<List<ImportBatch>> history(String before) async => [fixtureBatch()];
  @override
  Future<ImportBatch> batch(String id) async => fixtureBatch();
  @override
  Future<TransactionRecord> transaction(int id) async =>
      const TransactionRecord(item, item, true, 7, 0);
  @override
  Future<List<TransactionAudit>> audit(String target) async => [];
  @override
  Future<MutationResult> mutate(MutationRequest request) async {
    requests.add(request);
    if (reject != null) throw reject!;
    if (delayed != null) return delayed!.future;
    saved = MutationResult(request.id, 1, 1, 1);
    if (loseResponse) throw TimeoutException('response lost');
    return saved!;
  }

  @override
  Future<MutationResult> result(String id) async {
    if (saved == null) throw const ApiException('not found', statusCode: 404);
    return saved!;
  }
}

// Avoid collision between the repository method and top-level fixture name.
ProviderContainer containerFor(ManagementFake repository) {
  final fakes = FakeRepositories();
  return ProviderContainer(
    overrides: [
      importManagementRepositoryProvider.overrideWithValue(repository),
      transactionsRepositoryProvider.overrideWithValue(fakes.transactions),
      dashboardRepositoryProvider.overrideWithValue(fakes.dashboard),
    ],
  );
}

class ReactivationFake extends FakeTransactionsRepository {
  final confirmations = <bool>[];
  @override
  Future<ImportPreview> previewImport({
    bool confirmReactivation = false,
    required String filePath,
    required String fileName,
    required String source,
    required String accountName,
    CsvMapping? mapping,
    List<ImportIdentityDecision> identityDecisions = const [],
    List<PurchaseCsvDecision> reconciliationDecisions = const [],
  }) async {
    confirmations.add(confirmReactivation);
    final base = await super.previewImport(
      filePath: filePath,
      fileName: fileName,
      source: source,
      accountName: accountName,
      mapping: mapping,
    );
    return ImportPreview(
      reactivationIds: const [1],
      previewId: confirmReactivation ? 'restored-preview' : '',
      detectedEncoding: base.detectedEncoding,
      headers: base.headers,
      mapping: base.mapping,
      needsMapping: false,
      readCount: base.readCount,
      periodStart: base.periodStart,
      periodEnd: base.periodEnd,
      expenseTotal: base.expenseTotal,
      rows: base.rows,
      warnings: base.warnings,
    );
  }
}

class PagedManagementFake extends ManagementFake {
  final cursors = <String>[];
  @override
  Future<List<ImportBatch>> history(String before) async {
    cursors.add(before);
    return [
      for (var i = 0; i < (before.isEmpty ? 20 : 1); i++)
        ImportBatch(
          id: before.isEmpty ? 'batch-$i' : 'older-batch',
          fileName: before.isEmpty ? 'new-$i.csv' : 'older.csv',
          source: 'bank',
          account: 'account',
          status: 'active',
          version: 1,
          stateVersion: 7,
          snapshot: '',
          recordedAt: '',
          legacy: true,
          affected: 0,
          protected: 0,
          rows: const [],
        ),
    ];
  }
}

void main() {
  test(
    'DTO preserves bank original/effective amount and exact mutation concurrency fields',
    () {
      final source = {
        'id': 1,
        'transaction_date': '2035-01-01',
        'amount': 2500,
        'revision': 0,
      };
      final corrected = {...source, 'amount': 1800, 'revision': 2};
      final record = transactionRecordFromJson({
        'original': source,
        'current': corrected,
        'active': false,
        'state_version': 9,
      });
      expect(record.original.amount, 2500);
      expect(record.current.amount, 1800);
      expect(record.revision, 2);
      expect(record.active, false);
      final request = mutationRequestToJson(
        const MutationRequest(
          id: 'same-logical-operation',
          target: '1',
          action: 'correct',
          version: 2,
          stateVersion: 9,
          amount: 1800,
          date: '2035-02-01',
          category: '自由支出',
        ),
      );
      expect(request['expected_version'], 2);
      expect(request['state_version'], 9);
      expect(request['amount'], 1800);
      expect(request.containsKey('fingerprint'), false);
      final audit = transactionAuditFromJson({
        'kind': 'correct',
        'recorded_at': '2035-02-01T00:00:00Z',
        'before_state': jsonEncode([source]),
        'after_state': jsonEncode([corrected]),
      });
      expect(audit.before.single.amount, 2500);
      expect(audit.after.single.amount, 1800);
    },
  );
  test(
    'lost mutation response recovers saved result without another write',
    () async {
      final repository = ManagementFake()..loseResponse = true;
      final container = containerFor(repository);
      addTearDown(container.dispose);
      const key = (id: 'batch', isImport: true);
      final keep = container.listen(
        managementControllerProvider(key),
        (_, _) {},
      );
      addTearDown(keep.close);
      await Future<void>.delayed(Duration.zero);
      final vm = container.read(managementControllerProvider(key).notifier);
      const request = MutationRequest(
        id: 'recover-one',
        target: 'batch',
        action: 'cancel',
        version: 1,
        stateVersion: 7,
        snapshot: 'same-snapshot',
      );
      await vm.dispatch(const SaveManagement(request));
      expect(
        container.read(managementControllerProvider(key)).pending?.id,
        'recover-one',
      );
      expect(repository.requests.length, 1);
      await vm.dispatch(const RecoverManagement());
      expect(repository.requests.length, 1);
      expect(container.read(managementControllerProvider(key)).pending, isNull);
      expect(
        container.read(managementControllerProvider(key)).result?.id,
        'recover-one',
      );
      expect(
        container.read(managementControllerProvider(key)).result?.pending,
        1,
      );
    },
  );
  test(
    'definite version rejection unlocks original input; late disposed response is not published',
    () async {
      final repository = ManagementFake()
        ..reject = const ApiException('changed', statusCode: 409);
      final container = containerFor(repository);
      addTearDown(container.dispose);
      const key = (id: '1', isImport: false);
      final keep = container.listen(
        managementControllerProvider(key),
        (_, _) {},
      );
      await Future<void>.delayed(Duration.zero);
      final vm = container.read(managementControllerProvider(key).notifier);
      const request = MutationRequest(
        id: 'correct-one',
        target: '1',
        action: 'correct',
        version: 0,
        stateVersion: 7,
        amount: 1800,
      );
      await vm.dispatch(const SaveManagement(request));
      expect(container.read(managementControllerProvider(key)).pending, isNull);
      expect(
        container
            .read(managementControllerProvider(key))
            .record
            ?.current
            .amount,
        2500,
      );
      expect(
        container.read(managementControllerProvider(key)).error,
        'changed',
      );
      repository.reject = null;
      repository.delayed = Completer<MutationResult>();
      final work = vm.dispatch(const SaveManagement(request));
      await Future<void>.delayed(Duration.zero);
      keep.close();
      container.invalidate(managementControllerProvider(key));
      repository.delayed!.complete(
        const MutationResult('correct-one', 1, 0, 1),
      );
      await work;
    },
  );
  testWidgets(
    'bank cancellation asks explicitly and keeps latest purchase separate',
    (tester) async {
      final repository = ManagementFake();
      final fakes = FakeRepositories();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            importManagementRepositoryProvider.overrideWithValue(repository),
            transactionsRepositoryProvider.overrideWithValue(
              fakes.transactions,
            ),
            dashboardRepositoryProvider.overrideWithValue(fakes.dashboard),
          ],
          child: const MaterialApp(
            home: ManagementScreen(id: 'batch', isImport: true),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('取込日時不明'), findsOneWidget);
      await tester.ensureVisible(find.text('この取込を取り消す'));
      await tester.tap(find.text('この取込を取り消す'));
      await tester.pumpAndSettle();
      expect(repository.requests, isEmpty);
      expect(find.textContaining('本人の購入は取り消しません'), findsOneWidget);
      await tester.tap(find.text('戻る'));
      await tester.pumpAndSettle();
      expect(repository.requests, isEmpty);
      await tester.tap(find.text('この取込を取り消す'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('変更を保存'));
      await tester.pumpAndSettle();
      expect(repository.requests.single.snapshot, 'same-snapshot');
      expect(repository.requests.single.stateVersion, 7);
      expect(find.textContaining('残高・自由予算への反映は次の相談'), findsOneWidget);
    },
  );
  test(
    'reactivation consent is explicit and invalidated by source/account change',
    () async {
      final repository = ReactivationFake();
      final container = ProviderContainer(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.selectAndPreview(
        filePath: '/tmp/fixture.csv',
        fileName: 'fixture.csv',
      );
      expect(
        container.read(importControllerProvider).preview!.previewId,
        isEmpty,
      );
      expect(
        container.read(importControllerProvider).confirmReactivation,
        false,
      );
      await vm.dispatch(const ConfirmImportReactivation());
      expect(repository.confirmations, [false, true]);
      expect(
        container.read(importControllerProvider).preview!.previewId,
        'restored-preview',
      );
      expect(
        container.read(importControllerProvider).confirmReactivation,
        true,
      );
      await vm.setSource('another-bank', 'another-account');
      expect(repository.confirmations.last, false);
      expect(
        container.read(importControllerProvider).confirmReactivation,
        false,
      );
      expect(
        container.read(importControllerProvider).preview!.previewId,
        isEmpty,
      );
    },
  );
  testWidgets(
    'correction validates dates and preserves entered values on unknown result',
    (tester) async {
      final repository = ManagementFake()..loseResponse = true;
      final fakes = FakeRepositories();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            importManagementRepositoryProvider.overrideWithValue(repository),
            transactionsRepositoryProvider.overrideWithValue(
              fakes.transactions,
            ),
            dashboardRepositoryProvider.overrideWithValue(fakes.dashboard),
          ],
          child: const MaterialApp(
            home: ManagementScreen(id: '1', isImport: false),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final fields = find.byType(TextField);
      await tester.enterText(fields.at(0), '1800');
      await tester.enterText(fields.at(1), '2035-02-30');
      await tester.ensureVisible(find.text('訂正を保存'));
      await tester.tap(find.text('訂正を保存'));
      await tester.pumpAndSettle();
      expect(repository.requests, isEmpty);
      await tester.enterText(fields.at(1), '2035-02-01');
      await tester.ensureVisible(find.text('訂正を保存'));
      await tester.tap(find.text('訂正を保存'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('変更を保存'));
      await tester.pumpAndSettle();
      expect(repository.requests.single.amount, 1800);
      expect(repository.requests.single.date, '2035-02-01');
      expect(tester.widget<TextField>(fields.at(0)).controller!.text, '1800');
      expect(tester.widget<TextField>(fields.at(0)).enabled, false);
      await tester.ensureVisible(find.text('保存結果を確認'));
      await tester.tap(find.text('保存結果を確認'));
      await tester.pumpAndSettle();
      expect(repository.requests.length, 1);
      expect(tester.widget<TextField>(fields.at(0)).enabled, true);
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets('twenty-row history page exposes next page using the final id', (
    tester,
  ) async {
    final repository = PagedManagementFake();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          importManagementRepositoryProvider.overrideWithValue(repository),
        ],
        child: const MaterialApp(home: ImportHistoryScreen()),
      ),
    );
    await tester.pumpAndSettle();
    expect(repository.cursors, ['']);
    await tester.scrollUntilVisible(find.text('前の取込を見る'), 500);
    await tester.tap(find.text('前の取込を見る'));
    await tester.pumpAndSettle();
    expect(repository.cursors, ['', 'batch-19']);
    expect(find.text('older.csv'), findsOneWidget);
    expect(find.text('前の取込を見る'), findsNothing);
  });
  test('audit labels describe user operations in Japanese', () {
    expect(auditLabel('correct'), '訂正');
    expect(auditLabel('cancel'), '取込取消');
    expect(auditLabel('restore'), '再有効化');
    expect(auditLabel('financial_confirmation'), '金融反映の確認');
  });
}
