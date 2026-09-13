import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:spendable_today/features/dashboard/screens/dashboard/dashboard_view_model.dart';
import 'package:spendable_today/features/dashboard/screens/dashboard/dashboard_intent.dart';
import 'package:spendable_today/features/transactions/repository/transactions_repository.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import 'transactions_intent.dart';
import 'transactions_state.dart';

@riverpod
TransactionsViewModel transactionsViewModelFactory(Ref ref) =>
    TransactionsViewModel(ref);

final transactionsViewModelProvider =
    StateNotifierProvider<TransactionsViewModel, TransactionsState>(
      transactionsViewModelFactory,
    );

class TransactionsViewModel
    extends MviViewModel<TransactionsState, TransactionsIntent> {
  TransactionsViewModel(this.ref) : super(TransactionsState.initial()) {
    dispatch(const LoadTransactions());
  }

  final Ref ref;

  @override
  Future<void> dispatch(TransactionsIntent intent) {
    switch (intent) {
      case LoadTransactions():
      case RefreshTransactions():
        return _load();
      case ChangeTransactionCategory(
        id: final id,
        category: final category,
        rememberMerchant: final rememberMerchant,
      ):
        return _updateCategory(
          id,
          category,
          rememberMerchant: rememberMerchant,
        );
    }
  }

  Future<void> _load() async {
    state = state.copyWith(data: const AsyncLoading());
    try {
      final loaded = await ref.read(transactionsRepositoryProvider).getAll();
      if (!mounted) return;
      state = state.copyWith(data: AsyncData(loaded));
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = state.copyWith(data: AsyncError(error, stackTrace));
    }
  }

  Future<void> _updateCategory(
    int id,
    String category, {
    required bool rememberMerchant,
  }) async {
    try {
      final updated = await ref
          .read(transactionsRepositoryProvider)
          .updateCategory(id, category, rememberMerchant: rememberMerchant);
      if (!mounted) return;
      final current = state.data.valueOrNull;
      if (current != null) {
        state = state.copyWith(
          data: AsyncData([
            for (final item in current)
              if (item.id == updated.id) updated else item,
          ]),
        );
      }
      ref
          .read(dashboardViewModelProvider.notifier)
          .dispatch(RefreshDashboard());
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = state.copyWith(data: AsyncError(error, stackTrace));
      rethrow;
    }
  }
}
