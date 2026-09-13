import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:spendable_today/features/dashboard/domain/dashboard.dart';
import 'package:spendable_today/features/dashboard/screens/dashboard/dashboard_intent.dart';
import 'package:spendable_today/features/dashboard/screens/dashboard/dashboard_view_model.dart';
import 'package:spendable_today/shared/utils/formatters.dart';
import 'package:spendable_today/shared/widgets/async_error_card.dart';
import 'package:spendable_today/shared/widgets/section_title.dart';

class DashboardScreen extends ConsumerWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final dashboard = ref.watch(dashboardViewModelProvider).data;
    return RefreshIndicator(
      onRefresh: () => ref
          .read(dashboardViewModelProvider.notifier)
          .dispatch(const RefreshDashboard()),
      child: dashboard.when(
        loading: () => ListView(
          children: [
            SizedBox(height: 280),
            Center(child: CircularProgressIndicator()),
          ],
        ),
        error: (error, _) => ListView(
          padding: const EdgeInsets.all(20),
          children: [
            AsyncErrorCard(
              error: error,
              onRetry: () => ref
                  .read(dashboardViewModelProvider.notifier)
                  .dispatch(const RefreshDashboard()),
            ),
          ],
        ),
        data: (data) => _DashboardBody(data: data),
      ),
    );
  }
}

class _DashboardBody extends StatelessWidget {
  const _DashboardBody({required this.data});

  final Dashboard data;

  @override
  Widget build(BuildContext context) {
    final progress = data.monthlyFreeBudget == 0
        ? 0.0
        : (data.freeExpenseTotal / data.monthlyFreeBudget).clamp(0.0, 1.0);
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
      children: [
        Text(
          '今月の意思決定',
          style: Theme.of(
            context,
          ).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w800),
        ),
        const SizedBox(height: 14),
        Card(
          color: Theme.of(context).colorScheme.primary,
          child: Padding(
            padding: const EdgeInsets.all(22),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '自由予算の残り',
                  style: TextStyle(
                    color: Theme.of(
                      context,
                    ).colorScheme.onPrimary.withValues(alpha: 0.8),
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  yen(data.freeBudgetRemaining),
                  key: const ValueKey('free-budget-remaining'),
                  style: Theme.of(context).textTheme.displaySmall?.copyWith(
                    color: Theme.of(context).colorScheme.onPrimary,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 14),
                LinearProgressIndicator(
                  value: progress,
                  minHeight: 8,
                  borderRadius: BorderRadius.circular(8),
                  backgroundColor: Colors.white24,
                  color: Colors.white,
                ),
                const SizedBox(height: 8),
                Text(
                  '${yen(data.monthlyFreeBudget)}中 ${yen(data.freeExpenseTotal)}使用',
                  style: TextStyle(
                    color: Theme.of(
                      context,
                    ).colorScheme.onPrimary.withValues(alpha: 0.8),
                  ),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              child: _MetricCard(
                label: '今月の支出',
                value: yen(data.expenseTotal),
                icon: Icons.payments_outlined,
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: _MetricCard(
                label: '酒・飲み会',
                value: yen(data.drinkingTotal),
                icon: Icons.local_bar_outlined,
              ),
            ),
          ],
        ),
        const SizedBox(height: 10),
        Row(
          children: [
            Expanded(
              child: _MetricCard(
                label: 'サブスク',
                value: yen(data.subscriptionTotal),
                icon: Icons.autorenew,
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: _MetricCard(
                label: '固定費推定',
                value: yen(data.fixedExpenseEstimate),
                icon: Icons.home_work_outlined,
              ),
            ),
          ],
        ),
        const SizedBox(height: 20),
        FilledButton.icon(
          key: const ValueKey('dashboard-consult'),
          onPressed: () => context.go('/consult'),
          icon: const Icon(Icons.chat_bubble_outline),
          label: const Padding(
            padding: EdgeInsets.symmetric(vertical: 13),
            child: Text('AIに相談する'),
          ),
        ),
        const SizedBox(height: 10),
        OutlinedButton.icon(
          onPressed: () => context.push('/review'),
          icon: const Icon(Icons.search_outlined),
          label: const Text('今月をレビューする'),
        ),
        const SizedBox(height: 10),
        OutlinedButton.icon(
          onPressed: () => context.push('/import'),
          icon: const Icon(Icons.upload_file_outlined),
          label: const Text('CSVを追加する'),
        ),
        const SizedBox(height: 24),
        const SectionTitle('直近の支出'),
        const SizedBox(height: 10),
        if (data.recentTransactions.isEmpty)
          const Card(
            child: Padding(
              padding: EdgeInsets.all(20),
              child: Text('CSVを取り込むと、ここに直近の支出が表示されます。'),
            ),
          )
        else
          Card(
            child: Column(
              children: [
                for (final item in data.recentTransactions)
                  ListTile(
                    title: Text(item.description),
                    subtitle: Text('${item.transactionDate}・${item.category}'),
                    trailing: Text(
                      yen(item.amount),
                      style: const TextStyle(fontWeight: FontWeight.w700),
                    ),
                  ),
              ],
            ),
          ),
      ],
    );
  }
}

class _MetricCard extends StatelessWidget {
  const _MetricCard({
    required this.label,
    required this.value,
    required this.icon,
  });

  final String label;
  final String value;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(icon, color: Theme.of(context).colorScheme.primary),
            const SizedBox(height: 16),
            Text(label, style: Theme.of(context).textTheme.labelMedium),
            const SizedBox(height: 3),
            Text(
              value,
              style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 18),
            ),
          ],
        ),
      ),
    );
  }
}
