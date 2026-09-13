import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/profile/domain/profile.dart';
import 'package:spendable_today/features/profile/repository/dto/profile_dto.dart';

void main() {
  test('profile DTO matches the server request and response contract', () {
    const profile = Profile(
      monthlyIncome: 400000,
      currentBalance: 1200000,
      monthlyFixedCosts: 180000,
      monthlyFreeBudget: 60000,
      monthlySavingsGoal: 80000,
      reduceCategories: ['酒・飲み会'],
      allowedCategories: ['学習', '健康'],
      longTermGoal: '1年で100万円貯める',
      adviceStrictness: 'バランス型',
    );

    final request = ProfileDto.fromDomain(profile).toJson();
    expect(request, {
      'monthly_income': 400000,
      'current_balance': 1200000,
      'monthly_fixed_costs': 180000,
      'monthly_free_budget': 60000,
      'monthly_savings_goal': 80000,
      'reduce_categories': ['酒・飲み会'],
      'allowed_categories': ['学習', '健康'],
      'long_term_goal': '1年で100万円貯める',
      'advice_strictness': 'バランス型',
    });

    final response = ProfileDto.fromJson({
      ...request,
      'updated_at': '2026-08-03T00:00:00Z',
    }).toDomain();
    expect(response.monthlyFreeBudget, profile.monthlyFreeBudget);
    expect(response.reduceCategories, profile.reduceCategories);
    expect(response.updatedAt, '2026-08-03T00:00:00Z');
  });
}
