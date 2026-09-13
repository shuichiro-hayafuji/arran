import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/shared/presentation/mvi.dart';

part 'transactions_intent.freezed.dart';

@freezed
sealed class TransactionsIntent with _$TransactionsIntent implements MviIntent {
  const TransactionsIntent._();

  const factory TransactionsIntent.load() = LoadTransactions;
  const factory TransactionsIntent.refresh() = RefreshTransactions;
  const factory TransactionsIntent.changeCategory(
    int id,
    String category, {
    required bool rememberMerchant,
  }) = ChangeTransactionCategory;
}
