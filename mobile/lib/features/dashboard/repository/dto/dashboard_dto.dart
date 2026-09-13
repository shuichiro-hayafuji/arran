import '../../../../shared/utils/json_parsing.dart';
import '../../../transactions/repository/dto/transaction_item_dto.dart';
import '../../domain/category_amount.dart';
import '../../domain/dashboard.dart';

class CategoryAmountDto {
  const CategoryAmountDto({required this.category, required this.amount});

  factory CategoryAmountDto.fromJson(Map<String, dynamic> json) =>
      CategoryAmountDto(
        category: json['category'] as String? ?? '',
        amount: parseInt(json['amount']),
      );

  final String category;
  final int amount;

  CategoryAmount toDomain() =>
      CategoryAmount(category: category, amount: amount);
}

class DashboardDto {
  const DashboardDto({
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

  factory DashboardDto.fromJson(Map<String, dynamic> json) => DashboardDto(
    month: json['month'] as String? ?? '',
    expenseTotal: parseInt(json['expense_total']),
    fixedExpenseEstimate: parseInt(json['fixed_expense_estimate']),
    freeExpenseTotal: parseInt(json['free_expense_total']),
    monthlyFreeBudget: parseInt(json['monthly_free_budget']),
    freeBudgetRemaining: parseInt(json['free_budget_remaining']),
    drinkingTotal: parseInt(json['drinking_total']),
    subscriptionTotal: parseInt(json['subscription_total']),
    byCategory: parseMaps(
      json['by_category'],
    ).map(CategoryAmountDto.fromJson).toList(),
    recentTransactions: parseMaps(
      json['recent_transactions'],
    ).map(TransactionItemDto.fromJson).toList(),
  );

  final String month;
  final int expenseTotal;
  final int fixedExpenseEstimate;
  final int freeExpenseTotal;
  final int monthlyFreeBudget;
  final int freeBudgetRemaining;
  final int drinkingTotal;
  final int subscriptionTotal;
  final List<CategoryAmountDto> byCategory;
  final List<TransactionItemDto> recentTransactions;

  Dashboard toDomain() => Dashboard(
    month: month,
    expenseTotal: expenseTotal,
    fixedExpenseEstimate: fixedExpenseEstimate,
    freeExpenseTotal: freeExpenseTotal,
    monthlyFreeBudget: monthlyFreeBudget,
    freeBudgetRemaining: freeBudgetRemaining,
    drinkingTotal: drinkingTotal,
    subscriptionTotal: subscriptionTotal,
    byCategory: byCategory.map((dto) => dto.toDomain()).toList(),
    recentTransactions: recentTransactions
        .map((dto) => dto.toDomain())
        .toList(),
  );
}
