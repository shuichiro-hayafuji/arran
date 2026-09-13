import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/transactions/domain/transaction_item.dart';

part 'transactions_state.freezed.dart';

@freezed
abstract class TransactionsState with _$TransactionsState {
  const factory TransactionsState({
    required AsyncValue<List<TransactionItem>> data,
  }) = _TransactionsState;

  factory TransactionsState.initial() =>
      const TransactionsState(data: AsyncLoading());

  AsyncValue<List<TransactionItem>> get data;
}
