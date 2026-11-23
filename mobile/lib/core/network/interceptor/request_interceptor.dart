import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../features/session/provider/session_provider.dart';

/// An operation bound to a login must not borrow a later login's token.
const expectedSessionGenerationKey = 'arran.expectedSessionGeneration';

/// 認証必須の通信には送信時点のトークンを付与する。
/// トークンがなければネットワークへ送らず、401 相当のエラーにする。
void requestInterceptor(
  RequestOptions options,
  RequestInterceptorHandler handler,
  bool needAuthorize,
  Ref ref,
) {
  if (needAuthorize) {
    final session = ref.read(sessionControllerProvider);
    final expected = options.extra[expectedSessionGenerationKey];
    if (expected != null &&
        (expected is! int || expected != session.generation)) {
      handler.reject(
        DioException(
          requestOptions: options,
          type: DioExceptionType.badResponse,
          response: Response<dynamic>(
            requestOptions: options,
            statusCode: 409,
            data: {'message': 'ログイン状態が変更されました。'},
          ),
        ),
      );
      return;
    }
    final token = session.token;
    if (token == null) {
      handler.reject(
        DioException(
          requestOptions: options,
          type: DioExceptionType.badResponse,
          response: Response<dynamic>(
            requestOptions: options,
            statusCode: 401,
            data: {'message': 'ログインしてください。'},
          ),
        ),
      );
      return;
    }
    options.headers['Authorization'] = 'Bearer $token';
  }
  handler.next(options);
}
