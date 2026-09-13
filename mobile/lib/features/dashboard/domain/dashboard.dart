import '../../transactions/domain/transaction_item.dart';
import 'category_amount.dart';

class Dashboard {
  const Dashboard({
    required this.month,
    required this.expenseTotal,
    required this.fixedExpenseEstimate,
    required this.freeExpenseTotal,
    required this.monthlyFreeBudget,
    required this.freeBudgetRemaining,
    required this.drinkingTotal,
    required this.subscriptionTotal,
    required this.byCategory,
    required this.recentTransactions,
  });

  final String month;
  final int expenseTotal;
  final int fixedExpenseEstimate;
  final int freeExpenseTotal;
  final int monthlyFreeBudget;
  final int freeBudgetRemaining;
  final int drinkingTotal;
  final int subscriptionTotal;
  final List<CategoryAmount> byCategory;
  final List<TransactionItem> recentTransactions;
}
