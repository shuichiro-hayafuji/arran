import 'package:freezed_annotation/freezed_annotation.dart';

part 'app_exception.freezed.dart';

@freezed
abstract class AppException with _$AppException implements Exception {
  const factory AppException.error() = AppExceptionError;
  const factory AppException.initialize() = AppExceptionInitialize;
  const factory AppException.timeout() = AppExceptionTimeout;
}
