import 'package:dio/dio.dart';
import '../../../features/session/provider/session_provider.dart';

/// 401 を受けたリクエストのトークンを照合し、該当セッションだけを解除する。
Future<void> authenticationErrorInterceptor(
  DioException error,
  ErrorInterceptorHandler handler,
) async {
  if (error.response?.statusCode == 401) {
    final header = error.requestOptions.headers['Authorization'];
    if (header is String && header.startsWith('Bearer ')) {
      await authSession.clearIfCurrent(header.substring(7));
    }
  }
  handler.next(error);
}
