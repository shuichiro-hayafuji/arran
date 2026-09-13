import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:spendable_today/features/transactions/screens/list/transactions_intent.dart';
import 'package:spendable_today/features/transactions/providers/transactions_provider.dart';
import 'package:spendable_today/shared/utils/formatters.dart';
import 'package:spendable_today/shared/widgets/async_error_card.dart';

class TransactionsScreen extends ConsumerWidget {
  const TransactionsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final transactions = ref.watch(transactionsViewModelProvider).data;
    return RefreshIndicator(
      onRefresh: () => ref
          .read(transactionsViewModelProvider.notifier)
          .dispatch(const RefreshTransactions()),
      child: transactions.when(
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
                  .read(transactionsViewModelProvider.notifier)
                  .dispatch(const RefreshTransactions()),
            ),
          ],
        ),
        data: (items) => ListView(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    '取引',
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
                FilledButton.icon(
                  onPressed: () => context.push('/import'),
                  icon: const Icon(Icons.add),
                  label: const Text('CSV'),
                ),
              ],
            ),
            const SizedBox(height: 14),
            if (items.isEmpty)
              const Card(
                child: Padding(
                  padding: EdgeInsets.all(24),
                  child: Text('まだ取引がありません。CSVを追加してください。'),
                ),
              )
            else
              Card(
                child: Column(
                  children: [
                    for (final item in items) ...[
                      ListTile(
                        onTap: () => context.push('/transactions/${item.id}'),
                        title: Text(item.description),
                        subtitle: Text(
                          '${item.transactionDate}・${item.category}',
                        ),
                        trailing: Text(
                          '${item.transactionType == 'income' ? '+' : ''}${yen(item.amount)}',
                          style: TextStyle(
                            fontWeight: FontWeight.w700,
                            color: item.transactionType == 'income'
                                ? Colors.green
                                : null,
                          ),
                        ),
                      ),
                      if (item != items.last) const Divider(height: 1),
                    ],
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }
}
