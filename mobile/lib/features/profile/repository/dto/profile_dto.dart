import '../../../../shared/utils/json_parsing.dart';
import '../../domain/profile.dart';

class ProfileDto {
  const ProfileDto({
    required this.monthlyIncome,
    required this.currentBalance,
    required this.monthlyFixedCosts,
    required this.monthlyFreeBudget,
    required this.monthlySavingsGoal,
    required this.reduceCategories,
    required this.allowedCategories,
    required this.longTermGoal,
    required this.adviceStrictness,
    required this.updatedAt,
  });

  factory ProfileDto.fromJson(Map<String, dynamic> json) => ProfileDto(
    monthlyIncome: parseInt(json['monthly_income']),
    currentBalance: parseInt(json['current_balance']),
    monthlyFixedCosts: parseInt(json['monthly_fixed_costs']),
    monthlyFreeBudget: parseInt(json['monthly_free_budget']),
    monthlySavingsGoal: parseInt(json['monthly_savings_goal']),
    reduceCategories: parseStrings(json['reduce_categories']),
    allowedCategories: parseStrings(json['allowed_categories']),
    longTermGoal: json['long_term_goal'] as String? ?? '',
    adviceStrictness: json['advice_strictness'] as String? ?? 'バランス型',
    updatedAt: json['updated_at'] as String? ?? '',
  );

  factory ProfileDto.fromDomain(Profile profile) => ProfileDto(
    monthlyIncome: profile.monthlyIncome,
    currentBalance: profile.currentBalance,
    monthlyFixedCosts: profile.monthlyFixedCosts,
    monthlyFreeBudget: profile.monthlyFreeBudget,
    monthlySavingsGoal: profile.monthlySavingsGoal,
    reduceCategories: profile.reduceCategories,
    allowedCategories: profile.allowedCategories,
    longTermGoal: profile.longTermGoal,
    adviceStrictness: profile.adviceStrictness,
    updatedAt: profile.updatedAt,
  );

  final int monthlyIncome;
  final int currentBalance;
  final int monthlyFixedCosts;
  final int monthlyFreeBudget;
  final int monthlySavingsGoal;
  final List<String> reduceCategories;
  final List<String> allowedCategories;
  final String longTermGoal;
  final String adviceStrictness;
  final String updatedAt;

  /// 更新日時はサーバーが採番するため、保存リクエストには含めない。
  Map<String, dynamic> toJson() => {
    'monthly_income': monthlyIncome,
    'current_balance': currentBalance,
    'monthly_fixed_costs': monthlyFixedCosts,
    'monthly_free_budget': monthlyFreeBudget,
    'monthly_savings_goal': monthlySavingsGoal,
    'reduce_categories': reduceCategories,
    'allowed_categories': allowedCategories,
    'long_term_goal': longTermGoal,
    'advice_strictness': adviceStrictness,
  };

  Profile toDomain() => Profile(
    monthlyIncome: monthlyIncome,
    currentBalance: currentBalance,
    monthlyFixedCosts: monthlyFixedCosts,
    monthlyFreeBudget: monthlyFreeBudget,
    monthlySavingsGoal: monthlySavingsGoal,
    reduceCategories: reduceCategories,
    allowedCategories: allowedCategories,
    longTermGoal: longTermGoal,
    adviceStrictness: adviceStrictness,
    updatedAt: updatedAt,
  );
}
