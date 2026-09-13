import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../repository/transactions_repository.dart';

@riverpod
TransactionsRepository transactionsRepository(Ref ref) =>
    TransactionsRepositoryImpl(ref.watch(authorizedApiClientProvider));

final transactionsRepositoryProvider = Provider<TransactionsRepository>(
  transactionsRepository,
);
