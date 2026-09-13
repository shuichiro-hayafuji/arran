import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_screen.dart';
import 'package:spendable_today/features/consultation/domain/consultation.dart';
import 'package:spendable_today/features/consultation/repository/consultation_repository.dart';
import 'package:spendable_today/features/consultation/screens/result/result_screen.dart';
import 'package:spendable_today/features/dashboard/screens/dashboard/dashboard_screen.dart';
import 'package:spendable_today/features/dashboard/domain/dashboard.dart';
import 'package:spendable_today/features/dashboard/repository/dashboard_repository.dart';
import 'package:spendable_today/features/profile/screens/profile/profile_screen.dart';
import 'package:spendable_today/features/profile/domain/profile.dart';
import 'package:spendable_today/features/profile/domain/memory_item.dart';
import 'package:spendable_today/features/profile/repository/memory_repository.dart';
import 'package:spendable_today/features/transactions/screens/import/import_screen.dart';
import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/features/transactions/domain/import_commit_result.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/domain/transaction_item.dart';
import 'package:spendable_today/features/transactions/screens/import/import_view_model.dart';
import 'package:spendable_today/features/transactions/repository/transactions_repository.dart';

void main() {
  test('profile validation requires a positive free budget', () {
    expect(
      validateProfileValues(
        monthlyIncome: '400000',
        currentBalance: '1000000',
        monthlyFixedCosts: '180000',
        monthlyFreeBudget: '0',
        monthlySavingsGoal: '80000',
        strictness: 'バランス型',
      ),
      '月間の自由支出予算は必須です。',
    );
    expect(
      validateProfileValues(
        monthlyIncome: '400000',
        currentBalance: '1000000',
        monthlyFixedCosts: '180000',
        monthlyFreeBudget: '60000',
        monthlySavingsGoal: '80000',
        strictness: '厳しい',
      ),
      isNull,
    );
  });

  testWidgets('dashboard shows monthly totals', (tester) async {
    final fakes = FakeRepositories();
    await pumpFeature(tester, fakes, const DashboardScreen());
    await tester.pumpAndSettle();

    expect(find.text('¥41,910'), findsOneWidget);
    expect(find.text('¥18,090'), findsOneWidget);
    expect(find.byKey(const ValueKey('dashboard-consult')), findsOneWidget);
  });

  testWidgets('CSV import screen shows preview and commit states', (
    tester,
  ) async {
    final fakes = FakeRepositories();
    await pumpFeature(tester, fakes, const ImportScreen());
    final context = tester.element(find.byType(ImportScreen));
    final container = ProviderScope.containerOf(context);
    await container
        .read(importControllerProvider.notifier)
        .selectAndPreview(filePath: '/tmp/sample.csv', fileName: 'sample.csv');
    await tester.pumpAndSettle();

    expect(find.text('インポート前プレビュー'), findsOneWidget);
    expect(find.text('件数: 2件'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('commit-import')));
    await tester.pumpAndSettle();
    expect(find.text('2件を保存しました'), findsOneWidget);
    expect(find.text('重複 0件'), findsOneWidget);
  });

  testWidgets('consultation sends message and renders recommendation', (
    tester,
  ) async {
    final fakes = FakeRepositories();
    await pumpFeature(tester, fakes, const ConsultationScreen());
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('submit-consultation')));
    await tester.pumpAndSettle();

    expect(fakes.lastConsultationMessage, contains('飲みに行こう'));
    expect(find.byKey(const ValueKey('ai-recommendation')), findsOneWidget);
    expect(find.text('今回は見送ることをおすすめします。'), findsOneWidget);
  });

  testWidgets('consultation result is recorded', (tester) async {
    final fakes = FakeRepositories();
    await pumpFeature(tester, fakes, const ResultScreen(consultationId: 1));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('save-consultation-result')));
    await tester.pumpAndSettle();

    expect(fakes.lastResultStatus, 'skipped');
  });
}

Future<void> pumpFeature(
  WidgetTester tester,
  FakeRepositories fakes,
  Widget child,
) {
  return tester.pumpWidget(
    ProviderScope(
      overrides: [
        consultationRepositoryProvider.overrideWithValue(fakes.consultations),
        dashboardRepositoryProvider.overrideWithValue(fakes.dashboard),
        memoryRepositoryProvider.overrideWithValue(fakes.memories),
        transactionsRepositoryProvider.overrideWithValue(fakes.transactions),
      ],
      child: MaterialApp(home: child),
    ),
  );
}

class FakeRepositories {
  final consultations = FakeConsultationRepository();
  final dashboard = FakeDashboardRepository();
  final memories = FakeMemoryRepository();
  final transactions = FakeTransactionsRepository();

  String? get lastConsultationMessage => consultations.lastMessage;
  String? get lastResultStatus => consultations.lastStatus;
}

class FakeConsultationRepository implements ConsultationRepository {
  String? lastConsultationMessage;
  String? lastResultStatus;

  String? get lastMessage => lastConsultationMessage;
  String? get lastStatus => lastResultStatus;

  @override
  Future<Consultation> start(String message, int? plannedAmount) async {
    lastConsultationMessage = message;
    return testConsultation;
  }

  @override
  Future<Consultation> continueConversation(
    int id,
    String message,
    int? plannedAmount,
  ) async => testConsultation;

  @override
  Future<List<Consultation>> getAll() async => [testConsultation];

  @override
  Future<Consultation> updateResult(
    int id, {
    required String status,
    int? actualAmount,
    String reason = '',
    int? satisfaction,
    int? regret,
    String note = '',
  }) async {
    lastResultStatus = status;
    return testConsultation;
  }
}

class FakeDashboardRepository implements DashboardRepository {
  @override
  Future<Dashboard> getMonthly() async => testDashboard;
}

class FakeMemoryRepository implements MemoryRepository {
  @override
  Future<List<MemoryItem>> getAll() async => [];

  @override
  Future<MemoryItem> save(MemoryItem item) async => item;

  @override
  Future<void> delete(int id) async {}
}

class FakeTransactionsRepository implements TransactionsRepository {
  @override
  Future<List<TransactionItem>> getAll() async => testTransactions;

  @override
  Future<TransactionItem> updateCategory(
    int id,
    String category, {
    required bool rememberMerchant,
  }) async => testTransactions.first;

  @override
  Future<ImportPreview> previewImport({
    required String filePath,
    required String fileName,
    required String source,
    required String accountName,
    CsvMapping? mapping,
  }) async => ImportPreview(
    previewId: 'preview-1',
    detectedEncoding: 'UTF-8',
    headers: const ['date', 'description', 'amount'],
    mapping: const CsvMapping(
      dateColumn: 'date',
      descriptionColumn: 'description',
      amountColumn: 'amount',
    ),
    needsMapping: false,
    readCount: 2,
    periodStart: '2026-07-01',
    periodEnd: '2026-07-02',
    expenseTotal: 9490,
    rows: testTransactions,
    warnings: const [],
  );

  @override
  Future<ImportCommitResult> commitImport(String previewId) async =>
      const ImportCommitResult(importedCount: 2, duplicateCount: 0);
}

const testProfile = Profile(
  monthlyIncome: 400000,
  currentBalance: 1000000,
  monthlyFixedCosts: 180000,
  monthlyFreeBudget: 60000,
  monthlySavingsGoal: 80000,
  reduceCategories: ['酒・飲み会'],
  allowedCategories: ['学習'],
  longTermGoal: '1年で100万円貯める',
  adviceStrictness: 'バランス型',
);

const testTransactions = [
  TransactionItem(
    id: 1,
    transactionDate: '2026-07-02',
    description: '居酒屋',
    normalizedMerchant: '居酒屋',
    amount: 8000,
    transactionType: 'expense',
    category: '酒・飲み会',
    source: 'csv',
    sourceAccountName: '個人',
  ),
  TransactionItem(
    id: 2,
    transactionDate: '2026-07-03',
    description: 'NETFLIX',
    normalizedMerchant: 'netflix',
    amount: 1490,
    transactionType: 'expense',
    category: 'サブスクリプション',
    source: 'csv',
    sourceAccountName: '個人',
  ),
];

const testDashboard = Dashboard(
  month: '2026-07',
  expenseTotal: 18090,
  fixedExpenseEstimate: 0,
  freeExpenseTotal: 18090,
  monthlyFreeBudget: 60000,
  freeBudgetRemaining: 41910,
  drinkingTotal: 8000,
  subscriptionTotal: 1490,
  byCategory: [],
  recentTransactions: testTransactions,
);

const testConsultation = Consultation(
  id: 1,
  createdAt: '2026-07-31',
  userMessage: '今から飲みに行こうと思う。行っていい？',
  plannedAmount: 6000,
  inferredCategory: '酒・飲み会',
  recommendation: '今回は見送ることをおすすめします。',
  reasoningSummary: '減らしたいカテゴリで、今月すでに8,000円使っています。',
  currentSituation: '自由予算の残りは41,910円です。',
  alternative: '今日は自宅で食事を取る案があります。',
  finalQuestion: 'それでも今日行く価値がありますか？',
  status: 'awaiting_result',
  actualAmount: null,
  userDecisionReason: '',
  satisfactionScore: null,
  regretScore: null,
  note: '',
  responseSource: 'mock',
  needsFollowUp: false,
  followUpQuestion: '',
);
