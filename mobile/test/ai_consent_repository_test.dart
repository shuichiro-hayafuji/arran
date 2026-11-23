import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/core/network/api_client.dart';
import 'package:spendable_today/features/profile/repository/ai_consent_repository.dart';

class ConsentAdapter implements HttpClientAdapter {
  int code = 200;
  Map<String, dynamic> body = {};
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? stream,
    Future<void>? cancelFuture,
  ) async => ResponseBody.fromString(
    jsonEncode(body),
    code,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
  @override
  void close({bool force = false}) {}
}

void main() {
  late ConsentAdapter adapter;
  late AiConsentRepository repository;
  setUp(() {
    adapter = ConsentAdapter();
    final dio = Dio(
      BaseOptions(
        baseUrl: 'https://example.invalid',
        followRedirects: false,
        validateStatus: (status) => status != null && status < 400,
      ),
    );
    dio.httpClientAdapter = adapter;
    repository = AiConsentRepository(ApiClient(dio));
  });

  test('redirects never confirm a grant or withdrawal', () async {
    for (final code in [301, 307]) {
      adapter.code = code;
      adapter.body = {'status': 'withdrawn'};
      await expectLater(repository.withdraw(), throwsFormatException);
      adapter.body = {'status': 'granted'};
      await expectLater(repository.grant('v1'), throwsFormatException);
      await expectLater(repository.get(), throwsFormatException);
    }
  });

  test('mutation requires the matching explicit success state', () async {
    adapter.body = {'status': 'granted'};
    await repository.grant('v1');
    await expectLater(repository.withdraw(), throwsFormatException);
    adapter.body = {'status': 'withdrawn'};
    await repository.withdraw();
    await expectLater(repository.grant('v1'), throwsFormatException);
  });
}
