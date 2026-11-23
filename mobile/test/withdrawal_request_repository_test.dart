import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/core/network/api_client.dart';
import 'package:spendable_today/features/profile/repository/withdrawal_request_repository.dart';

class CapturingClient implements ApiClient {
  Options? captured;
  @override
  Future<Response<dynamic>> put(
    String path, {
    Object? data,
    Options? options,
  }) async {
    captured = options;
    return Response(
      requestOptions: RequestOptions(path: path),
      statusCode: 200,
      data: {'requested': true},
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  test('request carries the originating session generation', () async {
    final client = CapturingClient();
    await WithdrawalRequestRepository(client).submit(42);
    expect(client.captured?.extra?[expectedSessionGenerationKey], 42);
  });
}
