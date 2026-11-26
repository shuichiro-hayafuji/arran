import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart' show Options;
import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';

final withdrawalRequestRepositoryProvider =
    Provider<WithdrawalRequestRepository>(
      (ref) =>
          WithdrawalRequestRepository(ref.watch(authorizedApiClientProvider)),
    );
final withdrawalRequestProvider = FutureProvider.autoDispose<WithdrawalStatus>(
  (ref) => ref.watch(withdrawalRequestRepositoryProvider).get(),
);

class WithdrawalStatus {
  const WithdrawalStatus({required this.requested, required this.accepting});
  final bool requested;
  final bool accepting;
}

class WithdrawalRequestRepository {
  const WithdrawalRequestRepository(this._client);
  final ApiClient _client;
  Future<WithdrawalStatus> get() async {
    final response = await _client.get('/withdrawal-request');
    final data = parseMap(response.data);
    final value = data['requested'];
    final accepting = data['accepting'];
    if (response.statusCode != 200 || value is! bool || accepting is! bool)
      throw const FormatException();
    return WithdrawalStatus(requested: value, accepting: accepting);
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
