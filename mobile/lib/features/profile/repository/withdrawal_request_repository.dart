import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart' show Options;
import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';

final withdrawalRequestRepositoryProvider =
    Provider<WithdrawalRequestRepository>(
      (ref) =>
          WithdrawalRequestRepository(ref.watch(authorizedApiClientProvider)),
    );
final withdrawalRequestProvider = FutureProvider.autoDispose<bool>(
  (ref) => ref.watch(withdrawalRequestRepositoryProvider).get(),
);

class WithdrawalRequestRepository {
  const WithdrawalRequestRepository(this._client);
  final ApiClient _client;
  Future<bool> get() async {
    final response = await _client.get('/withdrawal-request');
    final value = parseMap(response.data)['requested'];
    if (response.statusCode != 200 || value is! bool)
      throw const FormatException();
    return value;
  }

  Future<void> submit(int generation) async {
    final response = await _client.put(
      '/withdrawal-request',
      options: Options(extra: {expectedSessionGenerationKey: generation}),
    );
    if (response.statusCode != 200 ||
        parseMap(response.data)['requested'] != true)
      throw const FormatException();
  }
}
