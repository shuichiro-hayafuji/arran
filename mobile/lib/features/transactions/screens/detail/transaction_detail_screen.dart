import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:spendable_today/features/transactions/domain/categories.dart';
import 'package:spendable_today/features/transactions/domain/transaction_item.dart';
import 'package:spendable_today/features/transactions/screens/list/transactions_intent.dart';
import 'package:spendable_today/features/transactions/providers/transactions_provider.dart';
import 'package:spendable_today/shared/utils/formatters.dart';

class TransactionDetailScreen extends ConsumerStatefulWidget {
  const TransactionDetailScreen({required this.transactionId, super.key});

  final int transactionId;

  @override
  ConsumerState<TransactionDetailScreen> createState() =>
      _TransactionDetailScreenState();
}

class _TransactionDetailScreenState
    extends ConsumerState<TransactionDetailScreen> {
  String? _category;
  var _remember = true;
  var _saving = false;

  Future<void> _save(TransactionItem item) async {
    setState(() => _saving = true);
    try {
      await ref
          .read(transactionsViewModelProvider.notifier)
          .dispatch(
            ChangeTransactionCategory(
              item.id,
              _category ?? item.category,
              rememberMerchant: _remember,
            ),
          );
      if (mounted) Navigator.pop(context);
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.toString())));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final items = ref.watch(transactionsViewModelProvider).data;
    return Scaffold(
      appBar: AppBar(title: const Text('取引詳細')),
      body: items.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(child: Text(error.toString())),
        data: (items) {
          final matches = items.where(
            (item) => item.id == widget.transactionId,
          );
          if (matches.isEmpty) return const Center(child: Text('取引が見つかりません'));
          final item = matches.first;
          _category ??= item.category;
          return ListView(
            padding: const EdgeInsets.all(20),
            children: [
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(20),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        item.description,
                        style: Theme.of(context).textTheme.titleLarge?.copyWith(
                          fontWeight: FontWeight.w800,
                        ),
                      ),
                      const SizedBox(height: 8),
                      Text(item.transactionDate),
                      const SizedBox(height: 16),
                      Text(
                        yen(item.amount),
                        style: Theme.of(context).textTheme.headlineMedium,
                      ),
                      const Divider(height: 32),
                      Text('正規化加盟店: ${item.normalizedMerchant}'),
                      Text('取込元: ${item.source} / ${item.sourceAccountName}'),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 16),
              DropdownButtonFormField<String>(
                initialValue: _category,
                decoration: const InputDecoration(labelText: 'カテゴリ'),
                items: categories
                    .map(
                      (category) => DropdownMenuItem(
                        value: category,
                        child: Text(category),
                      ),
                    )
                    .toList(),
                onChanged: (value) => setState(() => _category = value),
              ),
              CheckboxListTile(
                contentPadding: EdgeInsets.zero,
                value: _remember,
                onChanged: (value) =>
                    setState(() => _remember = value ?? false),
                title: const Text('同じ加盟店へ次回もこのカテゴリを適用'),
              ),
              const SizedBox(height: 12),
              FilledButton(
                onPressed: _saving ? null : () => _save(item),
                child: Padding(
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  child: Text(_saving ? '保存中…' : 'カテゴリを保存'),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}
