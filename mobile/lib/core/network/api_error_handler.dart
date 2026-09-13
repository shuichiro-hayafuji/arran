import 'package:dio/dio.dart';

import 'api_exception.dart';

/// Converts API transport failures into errors suitable for presentation.
extension ApiErrorHandler on DioException {
  ApiException get appException {
    final status = response?.statusCode;
    if (status == 404) {
      return const ApiException('データがまだ登録されていません。', statusCode: 404);
    }

    final data = response?.data;
    final serverMessage = data is Map ? data['message'] : null;
    if (serverMessage is String && serverMessage.isNotEmpty) {
      return ApiException(serverMessage, statusCode: status);
    }

    final message = switch (type) {
      DioExceptionType.connectionTimeout ||
      DioExceptionType.receiveTimeout ||
      DioExceptionType.sendTimeout => 'APIの応答がタイムアウトしました。',
      DioExceptionType.connectionError => 'Go APIに接続できません。起動状態を確認してください。',
      _ => '通信に失敗しました。再試行してください。',
    };
    return ApiException(message, statusCode: status);
  }
}
