import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:spendable_today/core/network/interceptor/request_interceptor.dart';
import 'package:spendable_today/core/network/interceptor/response_interceptor.dart';

import '../../config/env.dart';
import 'api_error_handler.dart';

export 'api_exception.dart';

final publicApiClientProvider = Provider<ApiClient>(
  (ref) => ApiClient.init(ref, needAuthorize: false),
);

final authorizedApiClientProvider = Provider<ApiClient>(
  (ref) => ApiClient.init(ref, needAuthorize: true),
);

/// 認証ヘッダーの付与と通信例外の変換を共通化する HTTP 窓口。
/// Dio は Provider の破棄時に閉じ、セッションをまたぐ通信を終了する。
class ApiClient {
  ApiClient(this._dio, {this.needAuthorize = true});

  factory ApiClient.init(
    Ref ref, {
    String? baseUrl,
    bool needAuthorize = true,
  }) {
    final dio = Dio(
      BaseOptions(
        connectTimeout: Duration(milliseconds: Env.connectTimeout),
        receiveTimeout: Duration(milliseconds: Env.receiveTimeout),
        sendTimeout: Duration(milliseconds: Env.sendTimeout),
        baseUrl: baseUrl ?? Env.apiBaseUrl,
        contentType: Headers.jsonContentType,
        followRedirects: false,
        validateStatus: (status) =>
            status != null && status < 400, // TODO エラー処理は全般見直す
      ),
    );

    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) =>
            requestInterceptor(options, handler, needAuthorize, ref),
        onError: authenticationErrorInterceptor,
      ),
    );

    ref.onDispose(() => dio.close(force: true));
    return ApiClient(dio, needAuthorize: needAuthorize);
  }

  final Dio _dio;
  final bool needAuthorize;

  String get baseUrl => _dio.options.baseUrl;

  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
  }) async {
    try {
      return await _dio.get<dynamic>(path, queryParameters: queryParameters);
    } on DioException catch (error) {
      Error.throwWithStackTrace(error.appException, error.stackTrace);
    }
  }

  Future<Response<dynamic>> post(
    String path, {
    Object? data,
    Options? options,
  }) async {
    try {
      return await _dio.post<dynamic>(path, data: data, options: options);
    } on DioException catch (error) {
      Error.throwWithStackTrace(error.appException, error.stackTrace);
    }
  }

  Future<Response<dynamic>> put(
    String path, {
    Object? data,
    Options? options,
  }) async {
    try {
      return await _dio.put<dynamic>(path, data: data, options: options);
    } on DioException catch (error) {
      Error.throwWithStackTrace(error.appException, error.stackTrace);
    }
  }

  Future<Response<dynamic>> patch(
    String path, {
    Object? data,
    Options? options,
  }) async {
    try {
      return await _dio.patch<dynamic>(path, data: data, options: options);
    } on DioException catch (error) {
      Error.throwWithStackTrace(error.appException, error.stackTrace);
    }
  }

  Future<Response<dynamic>> delete(
    String path, {
    Object? data,
    Options? options,
  }) async {
    try {
      return await _dio.delete<dynamic>(path, data: data, options: options);
    } on DioException catch (error) {
      Error.throwWithStackTrace(error.appException, error.stackTrace);
    }
  }
}
