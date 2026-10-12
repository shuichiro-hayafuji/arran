import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/repository/transactions_repository.dart';
import 'package:spendable_today/features/transactions/screens/import/import_intent.dart';
import 'package:spendable_today/features/transactions/screens/import/import_screen.dart';
import 'package:spendable_today/features/transactions/screens/import/import_view_model.dart';
import 'widget_test.dart' show FakeTransactionsRepository;

const headers = ['date', 'description', 'amount', 'debit', 'credit', 'id'];
const both = CsvMapping(
  dateColumn: 'date',
  descriptionColumn: 'description',
  amountColumn: 'amount',
  debitColumn: 'debit',
  creditColumn: 'credit',
  transactionIdColumn: 'id',
);
const split = CsvMapping(
  dateColumn: 'date',
  descriptionColumn: 'description',
  debitColumn: 'debit',
  creditColumn: 'credit',
  transactionIdColumn: 'id',
);

class MappingRepository extends FakeTransactionsRepository {
  MappingRepository(this.initial, {this.columns = headers});
  final CsvMapping initial;
  final List<String> columns;
  final requests = <CsvMapping?>[];
  final identities = <List<ImportIdentityDecision>>[];
  final purchases = <List<PurchaseCsvDecision>>[];
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
    requests.add(mapping);
    identities.add(identityDecisions);
    purchases.add(reconciliationDecisions);
    confirmations.add(confirmReactivation);
    return ImportPreview(
      previewId: 'preview-${requests.length}',
      detectedEncoding: 'UTF-8',
      headers: columns,
      mapping: mapping ?? initial,
      needsMapping: true,
      readCount: 0,
      periodStart: '',
      periodEnd: '',
      expenseTotal: 0,
      rows: const [],
      warnings: const [],
    );
  }
}

Future<ProviderContainer> openMapping(
  WidgetTester tester,
  MappingRepository repository, {
  double textScale = 1,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [transactionsRepositoryProvider.overrideWithValue(repository)],
      child: MaterialApp(
        home: MediaQuery(
          data: MediaQueryData(textScaler: TextScaler.linear(textScale)),
          child: const ImportScreen(),
        ),
      ),
    ),
  );
  final container = ProviderScope.containerOf(
    tester.element(find.byType(ImportScreen)),
  );
  await container
      .read(importControllerProvider.notifier)
      .dispatch(
        const SelectImportFile(
          filePath: '/tmp/mapping.csv',
          fileName: 'mapping.csv',
        ),
      );
  await tester.pumpAndSettle();
  return container;
}

Finder column(String label) => find.byWidgetPredicate(
  (widget) =>
      widget is DropdownButtonFormField<String> &&
      widget.decoration.labelText == label,
);

Future<void> selectMode(WidgetTester tester, String label) async {
  final finder = find.byKey(const ValueKey('mapping-amount-mode'));
  await tester.scrollUntilVisible(
    finder,
    200,
    scrollable: find.byType(Scrollable).first,
  );
  await tester.ensureVisible(finder);
  await tester.pumpAndSettle();
  await tester.tap(finder);
  await tester.pumpAndSettle();
  await tester.tap(find.text(label).last);
  await tester.pumpAndSettle();
}

Future<void> selectColumn(
  WidgetTester tester,
  String label,
  String value,
) async {
  await tester.ensureVisible(column(label));
  await tester.pumpAndSettle();
  await tester.tap(column(label));
  await tester.pumpAndSettle();
  await tester.tap(find.text(value).last);
  await tester.pumpAndSettle();
}

FilledButton applyButton(WidgetTester tester) =>
    tester.widget<FilledButton>(find.byKey(const ValueKey('apply-mapping')));

Future<void> applyMapping(WidgetTester tester) async {
  final finder = find.byKey(const ValueKey('apply-mapping'));
  await tester.ensureVisible(finder);
  await tester.pumpAndSettle();
  await tester.tap(finder);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('mapping stays operable at narrow width and large text', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final repository = MappingRepository(both);
    await openMapping(tester, repository, textScale: 2);
    await selectMode(tester, '出金・入金の別列');
    await applyMapping(tester);
    expect(repository.requests.last!.amountColumn, isEmpty);
    expect(repository.requests.last!.debitColumn, 'debit');
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'both candidates initially show single amount and submit only it',
    (tester) async {
      final repository = MappingRepository(both);
      await openMapping(tester, repository);
      expect(column('金額列'), findsOneWidget);
      expect(column('出金列'), findsNothing);
      expect(find.textContaining('残高ではなく'), findsOneWidget);
      await applyMapping(tester);
      final sent = repository.requests.last!;
      expect(sent.amountColumn, 'amount');
      expect(sent.debitColumn, isEmpty);
      expect(sent.creditColumn, isEmpty);
      expect(sent.transactionIdColumn, 'id');
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('single to split clears amount and preserves independent ID', (
    tester,
  ) async {
    final repository = MappingRepository(both);
    await openMapping(tester, repository);
    await selectMode(tester, '出金・入金の別列');
    expect(column('金額列'), findsNothing);
    await applyMapping(tester);
    final sent = repository.requests.last!;
    expect(sent.amountColumn, isEmpty);
    expect(sent.debitColumn, 'debit');
    expect(sent.creditColumn, 'credit');
    expect(sent.transactionIdColumn, 'id');
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'split starts selected; switching requires amount and clears split',
    (tester) async {
      final repository = MappingRepository(split);
      await openMapping(tester, repository);
      expect(column('出金列'), findsOneWidget);
      expect(applyButton(tester).onPressed, isNotNull);
      await selectMode(tester, '金額1列');
      expect(applyButton(tester).onPressed, isNull);
      await selectColumn(tester, '金額列', 'amount');
      await applyMapping(tester);
      final sent = repository.requests.last!;
      expect(sent.amountColumn, 'amount');
      expect(sent.debitColumn, isEmpty);
      expect(sent.creditColumn, isEmpty);
      expect(sent.transactionIdColumn, 'id');
      expect(tester.takeException(), isNull);
    },
  );

  for (final missing in ['date', 'description', 'amount']) {
    testWidgets('missing $missing header disables preview without assertion', (
      tester,
    ) async {
      final repository = MappingRepository(
        const CsvMapping(
          dateColumn: 'date',
          descriptionColumn: 'description',
          amountColumn: 'amount',
          transactionIdColumn: 'old-id',
        ),
        columns: headers.where((header) => header != missing).toList(),
      );
      await openMapping(tester, repository);
      expect(applyButton(tester).onPressed, isNull);
      expect(repository.requests.length, 1);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets(
    'invalid amount falls back to valid split; invalid ID is omitted',
    (tester) async {
      final repository = MappingRepository(
        const CsvMapping(
          dateColumn: 'date',
          descriptionColumn: 'description',
          amountColumn: 'old-amount',
          debitColumn: 'debit',
          creditColumn: 'old-credit',
          transactionIdColumn: 'old-id',
        ),
      );
      await openMapping(tester, repository);
      expect(column('出金列'), findsOneWidget);
      await selectColumn(tester, '出金列', '使用しない');
      expect(applyButton(tester).onPressed, isNull);
      await selectColumn(tester, '入金列', 'credit');
      await applyMapping(tester);
      final sent = repository.requests.last!;
      expect(sent.amountColumn, isEmpty);
      expect(sent.debitColumn, isEmpty);
      expect(sent.creditColumn, 'credit');
      expect(sent.transactionIdColumn, isEmpty);
      expect(tester.takeException(), isNull);
    },
  );

  test(
    'mapping repreview invalidates identity, purchase and reactivation consent',
    () async {
      final repository = MappingRepository(both);
      final container = ProviderContainer(
        overrides: [
          transactionsRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(importControllerProvider.notifier);
      await vm.dispatch(
        const SelectImportFile(
          filePath: '/tmp/mapping.csv',
          fileName: 'mapping.csv',
        ),
      );
      await vm.dispatch(
        const ResolveImportIdentities([
          ImportIdentityDecision(row: 2, action: 'distinct'),
        ]),
      );
      await vm.dispatch(
        const ResolveImportPurchases([
          PurchaseCsvDecision(row: 2, action: 'distinct', stateVersion: 7),
        ]),
      );
      await vm.dispatch(const ConfirmImportReactivation());
      final before = container.read(importControllerProvider);
      expect(before.identityDecisions, hasLength(1));
      expect(before.reconciliationDecisions, hasLength(1));
      expect(before.confirmReactivation, true);
      final oldId = before.preview!.previewId;
      await vm.dispatch(const ApplyImportMapping(split));
      final after = container.read(importControllerProvider);
      expect(after.preview!.previewId, isNot(oldId));
      expect(repository.requests.last, same(split));
      expect(repository.identities.last, isEmpty);
      expect(repository.purchases.last, isEmpty);
      expect(repository.confirmations.last, false);
      expect(after.identityDecisions, isEmpty);
      expect(after.reconciliationDecisions, isEmpty);
      expect(after.confirmReactivation, false);
    },
  );
}
