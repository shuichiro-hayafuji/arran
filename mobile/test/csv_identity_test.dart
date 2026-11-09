import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/repository/dto/import_dto.dart';
import 'package:spendable_today/features/transactions/repository/transactions_repository.dart';
import 'package:spendable_today/features/transactions/screens/import/import_screen.dart';
import 'package:spendable_today/features/transactions/screens/import/import_view_model.dart';
import 'package:spendable_today/features/transactions/screens/import/import_intent.dart';
import 'widget_test.dart' show FakeTransactionsRepository, testTransactions;

class IdentityRepository extends FakeTransactionsRepository {
  String? source;
  String? account;
  List<ImportIdentityDecision> decisions = [];
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
    this.source = source;
    account = accountName;
    decisions = identityDecisions;
    final p = await super.previewImport(
      filePath: filePath,
      fileName: fileName,
      source: source,
      accountName: accountName,
      mapping: mapping,
    );
    return ImportPreview(
      previewId: identityDecisions.isEmpty ? '' : 'resolved-preview',
      detectedEncoding: p.detectedEncoding,
      headers: p.headers,
      mapping: p.mapping,
      needsMapping: false,
      readCount: p.readCount,
      periodStart: p.periodStart,
      periodEnd: p.periodEnd,
      expenseTotal: p.expenseTotal,
      rows: p.rows,
      warnings: [],
      identityNotice: '別CSVの同条件明細を確認してください。',
      identityConflicts: identityDecisions.isEmpty
          ? [
              ImportIdentityConflict(
                row: 2,
                transaction: testTransactions.first,
                candidates: [testTransactions.first],
                candidateSnapshots: {testTransactions.first.id: 'a' * 64},
              ),
            ]
          : [],
    );
  }
}

void main() {
  test('CSV request carries scope, ID column and candidate snapshot', () async {
    final folder = Directory.systemTemp.createTempSync('arran-csv-dto-');
    addTearDown(() => folder.deleteSync(recursive: true));
    final file = File('${folder.path}/sample.csv')
      ..writeAsStringSync('date,amount\n2031-01-01,2500\n');
    final form = ImportPreviewRequestDto(
      filePath: file.path,
      fileName: 'sample.csv',
      source: '銀行A',
      accountName: '貯金口座',
      mapping: const CsvMapping(transactionIdColumn: '取引ID'),
      identityDecisions: [
        ImportIdentityDecision(
          row: 2,
          action: 'same',
          transactionId: 41,
          snapshot: 'a' * 64,
        ),
      ],
    ).toFormData();
    final values = Map.fromEntries(form.fields);
    expect(values['source_account_name'], '貯金口座');
    expect(
      (jsonDecode(values['mapping']!) as Map)['transaction_id_column'],
      '取引ID',
    );
    expect(
      (jsonDecode(values['identity_decisions']!) as List).single['snapshot'],
      'a' * 64,
    );
    final dto = ImportPreviewDto.fromJson({
      'identity_conflicts': [
        {
          'row': 2,
          'transaction': {'id': 0},
          'candidates': [
            {'id': 41},
          ],
          'candidate_snapshots': {'41': 'a' * 64},
        },
      ],
    });
    expect(
      dto.toDomain().identityConflicts.single.candidateSnapshots[41],
      'a' * 64,
    );
  });
  testWidgets('CSV source/account and explicit match enable commit', (
    tester,
  ) async {
    final r = IdentityRepository();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [transactionsRepositoryProvider.overrideWithValue(r)],
        child: const MaterialApp(home: ImportScreen()),
      ),
    );
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('import-source')), '銀行A');
    await tester.enterText(
      find.byKey(const ValueKey('import-account')),
      '普段口座',
    );
    await tester.tap(find.text('取込元・口座を適用して再確認'));
    await tester.pumpAndSettle();
    final c = ProviderScope.containerOf(
      tester.element(find.byType(ImportScreen)),
    );
    await c
        .read(importControllerProvider.notifier)
        .selectAndPreview(filePath: '/tmp/test.csv', fileName: 'test.csv');
    await tester.pumpAndSettle();
    expect(r.source, '銀行A');
    expect(r.account, '普段口座');
    expect(
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('commit-import')))
          .onPressed,
      isNull,
    );
    await tester.scrollUntilVisible(
      find.byKey(const ValueKey('identity-row-2')),
      250,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.tap(find.byKey(const ValueKey('identity-row-2')));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('既存 #').last);
    await tester.pumpAndSettle();
    await tester.ensureVisible(
      find.byKey(const ValueKey('resolve-import-identities')),
    );
    await tester.tap(find.byKey(const ValueKey('resolve-import-identities')));
    await tester.pumpAndSettle();
    expect(r.decisions.single.action, 'same');
    expect(r.decisions.single.transactionId, testTransactions.first.id);
    expect(r.decisions.single.snapshot, 'a' * 64);
    expect(
      c.read(importControllerProvider).preview!.previewId,
      'resolved-preview',
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('commit-import')))
          .onPressed,
      isNotNull,
    );
    await c
        .read(importControllerProvider.notifier)
        .dispatch(const SetImportSource('銀行A', '別口座'));
    await tester.pumpAndSettle();
    expect(r.account, '別口座');
    expect(r.decisions, isEmpty);
    expect(
      c.read(importControllerProvider).preview!.identityConflicts,
      isNotEmpty,
    );
    expect(tester.takeException(), isNull);
  });
}
