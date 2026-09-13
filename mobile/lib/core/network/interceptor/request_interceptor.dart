import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../features/session/provider/session_provider.dart';

/// 認証必須の通信には送信時点のトークンを付与する。
/// トークンがなければネットワークへ送らず、401 相当のエラーにする。
void requestInterceptor(
  RequestOptions options,
  RequestInterceptorHandler handler,
  bool needAuthorize,
  Ref ref,
) {
  if (needAuthorize) {
    final token = authSession.token;
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
