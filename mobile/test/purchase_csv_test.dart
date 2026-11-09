import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/domain/expense_count.dart';
import 'package:spendable_today/features/transactions/repository/dto/import_dto.dart';
import 'package:spendable_today/features/transactions/repository/reconciliation_repository.dart';
import 'package:spendable_today/features/transactions/screens/import/purchase_csv_controller.dart';
import 'package:spendable_today/features/transactions/screens/import/purchase_csv_form.dart';
import 'package:spendable_today/features/transactions/screens/import/purchase_csv_confirmation.dart';
import 'widget_test.dart' show testTransactions;

PurchaseCsvConflict conflict([int version = 7]) => PurchaseCsvConflict(
  row: 2,
  transaction: testTransactions.first,
  stateVersion: version,
  candidates: const [
    PurchaseCsvCandidate(
      id: 'p',
      description: 'headphones',
      amount: 2500,
      recordedAt: '2035-01-01',
      snapshot: 'local-snapshot',
    ),
  ],
);

class Repository implements ReconciliationRepository {
  Completer<List<PurchaseCsvConflict>>? pending;
  List<PurchaseCsvConflict> values = [conflict()];
  PurchaseCsvDecision? saved;
  int? transactionId;
  bool fail = false;
  @override
  Future<List<PurchaseCsvConflict>> candidates(String id) async =>
      pending == null ? values : pending!.future;
  @override
  Future<void> confirm(int id, PurchaseCsvDecision d) async {
    if (fail) throw Exception('stale choice');
    transactionId = id;
    saved = d;
    values = [];
  }

  @override
  Future<ExpenseCount> count() async => const ExpenseCount(
    confirmed: 1,
    estimated: 0,
    undated: 0,
    unresolved: 0,
    pendingRecalculation: 1,
  );
}

void main() {
  test(
    'expense count DTO retains separate dated subset and unresolved sets',
    () {
      final c = ExpenseCountDto.fromJson({
        'confirmed': 2,
        'estimated': 1,
        'undated': 3,
        'unresolved': 4,
        'pending_recalculation': 5,
      }).toDomain();
      expect(
        [
          c.confirmed,
          c.estimated,
          c.undated,
          c.unresolved,
          c.pendingRecalculation,
        ],
        [2, 1, 3, 4, 5],
      );
    },
  );
  test('decision preserves local identity snapshot without bank fields', () {
    expect(
      purchaseCsvDecisionToJson(
        const PurchaseCsvDecision(
          row: 2,
          action: 'same',
          purchaseId: 'p',
          snapshot: 'hash',
          stateVersion: 7,
        ),
      ),
      {
        'row': 2,
        'action': 'same',
        'purchase_id': 'p',
        'snapshot': 'hash',
        'state_version': 7,
      },
    );
    final parsed = purchaseCsvConflictFromJson({
      'row': 2,
      'state_version': 7,
      'transaction': {'id': 41, 'description': 'local CSV'},
      'candidates': [
        {'id': 'p', 'actual_amount': 2500, 'snapshot': 'hash'},
      ],
    });
    expect(parsed.transaction.description, 'local CSV');
    expect(parsed.candidates.single.amount, 2500);
    expect(parsed.candidates.single.purchaseDate, isEmpty);
  });
  testWidgets('preview requires choice and invalidates stale version', (
    tester,
  ) async {
    List<PurchaseCsvDecision>? selected;
    Future<void> show(int v) => tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: PurchaseCsvForm(
            conflicts: [conflict(v)],
            onSubmit: (d) => selected = d,
          ),
        ),
      ),
    );
    await show(7);
    await tester.pumpAndSettle();
    final button = find.byType(FilledButton);
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('headphones').last);
    await tester.pumpAndSettle();
    await tester.tap(button);
    expect(selected!.single.purchaseId, 'p');
    expect(selected!.single.snapshot, 'local-snapshot');
    await show(8);
    await tester.pumpAndSettle();
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('どの購入とも別の支出').last);
    await tester.pumpAndSettle();
    await tester.tap(button);
    expect(selected!.single.action, 'distinct');
    expect(selected!.single.purchaseId, isEmpty);
  });
  test('session disposal discards late candidates', () async {
    final r = Repository()..pending = Completer<List<PurchaseCsvConflict>>();
    final c = ProviderContainer(
      overrides: [reconciliationRepositoryProvider.overrideWithValue(r)],
    );
    final sub = c.listen(purchaseCsvControllerProvider('p'), (_, __) {});
    await Future<void>.delayed(Duration.zero);
    sub.close();
    c.dispose();
    r.pending!.complete([conflict()]);
    await Future<void>.delayed(Duration.zero);
  });
  testWidgets('chat saves distinct choice and leaves money for next chat', (
    tester,
  ) async {
    final r = Repository();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [reconciliationRepositoryProvider.overrideWithValue(r)],
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
    await tester.tap(find.text('どのCSV明細とも別の購入').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('同一性を確認して保存'));
    await tester.pumpAndSettle();
    expect(r.transactionId, 0);
    expect(r.saved!.snapshot, 'local-snapshot');
    expect(find.text('照合を保存しました。金融反映は次のチャットで確定します。'), findsOneWidget);
  });
  test('failed confirmation keeps candidates until explicit reload', () async {
    final r = Repository()..fail = true;
    final c = ProviderContainer(
      overrides: [reconciliationRepositoryProvider.overrideWithValue(r)],
    );
    addTearDown(c.dispose);
    final sub = c.listen(purchaseCsvControllerProvider('p'), (_, __) {});
    addTearDown(sub.close);
    await Future<void>.delayed(Duration.zero);
    await c
        .read(purchaseCsvControllerProvider('p').notifier)
        .dispatch(
          const ConfirmPurchaseCsv(
            41,
            PurchaseCsvDecision(
              row: 2,
              action: 'same',
              stateVersion: 7,
              purchaseId: 'p',
              snapshot: 'hash',
            ),
          ),
        );
    final state = c.read(purchaseCsvControllerProvider('p'));
    expect(state.error, isNotNull);
    expect(state.candidates, hasLength(1));
    expect(state.saved, isFalse);
  });
}
