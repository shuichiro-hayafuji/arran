import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../screens/list/transactions_view_model.dart';
import '../screens/list/transactions_state.dart';

@riverpod
TransactionsViewModel transactionsViewModelFactory(Ref ref) =>
    TransactionsViewModel(ref);

final transactionsViewModelProvider =
    StateNotifierProvider<TransactionsViewModel, TransactionsState>(
      transactionsViewModelFactory,
    );
