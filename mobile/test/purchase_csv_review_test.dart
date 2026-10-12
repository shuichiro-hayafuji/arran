import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/repository/reconciliation_repository.dart';
import 'package:spendable_today/features/transactions/screens/import/purchase_csv_controller.dart';
import 'package:spendable_today/features/transactions/screens/import/purchase_csv_confirmation.dart';
import 'package:spendable_today/features/transactions/screens/import/purchase_csv_form.dart';
import 'purchase_csv_test.dart' show Repository, conflict;
import 'widget_test.dart' show testTransactions;

void main() {
  testWidgets('review chat confirmation clears selection after revision reload', (
    tester,
  ) async {
    final r = Repository();
    final c = ProviderContainer(
      overrides: [reconciliationRepositoryProvider.overrideWithValue(r)],
    );
    addTearDown(c.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: c,
        child: const MaterialApp(
          home: Scaffold(
            body: PurchaseCsvConfirmation(
              purchaseId: 'p',
              description: 'headphones',
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<int>));
    await tester.pumpAndSettle();
    await tester.tap(
      find.textContaining(testTransactions.first.description).last,
    );
    await tester.pumpAndSettle();
    r.values = [conflict(8)];
    await c
        .read(purchaseCsvControllerProvider('p').notifier)
        .dispatch(const LoadPurchaseCsv());
    await tester.pumpAndSettle();
    final b = tester.widget<FilledButton>(find.byType(FilledButton));
    expect(
      b.onPressed,
      isNull,
      reason:
          'old choice must not be rebased onto new snapshot/version without confirmation',
    );
    await tester.tap(find.byType(DropdownButtonFormField<int>));
    await tester.pumpAndSettle();
    await tester.tap(
      find.textContaining(testTransactions.first.description).last,
    );
    await tester.pumpAndSettle();
    expect(
      tester.widget<FilledButton>(find.byType(FilledButton)).onPressed,
      isNotNull,
    );
    final previous = conflict(8);
    r.values = [
      PurchaseCsvConflict(
        row: previous.row,
        transaction: previous.transaction,
        stateVersion: 8,
        candidates: const [
          PurchaseCsvCandidate(
            id: 'p',
            description: 'headphones',
            amount: 2500,
            recordedAt: '2035-01-01',
            snapshot: 'changed-snapshot',
          ),
        ],
      ),
    ];
    await c
        .read(purchaseCsvControllerProvider('p').notifier)
        .dispatch(const LoadPurchaseCsv());
    await tester.pumpAndSettle();
    expect(
      tester.widget<FilledButton>(find.byType(FilledButton)).onPressed,
      isNull,
      reason: 'snapshot-only changes must invalidate the choice too',
    );
  });
  testWidgets('review confirmation form fits narrow screen enlarged text', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(
      MaterialApp(
        home: MediaQuery(
          data: const MediaQueryData(textScaler: TextScaler.linear(2)),
          child: Scaffold(
            body: SingleChildScrollView(
              child: PurchaseCsvForm(conflicts: [conflict()], onSubmit: (_) {}),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
}
