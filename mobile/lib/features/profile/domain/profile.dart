class Profile {
  const Profile({
    required this.monthlyIncome,
    required this.currentBalance,
    required this.monthlyFixedCosts,
    required this.monthlyFreeBudget,
    required this.monthlySavingsGoal,
    required this.reduceCategories,
    required this.allowedCategories,
    required this.longTermGoal,
    required this.adviceStrictness,
    this.updatedAt = '',
  });

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
}
