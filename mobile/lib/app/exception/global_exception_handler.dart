import 'dart:async';
import 'dart:ui';

import 'package:flutter/cupertino.dart';
import 'package:spendable_today/app/exception/app_exception.dart';
import 'package:spendable_today/app/router/router.dart';

extension AppExceptionHandler on AppException {
  Future<void> _handle(BuildContext context) async {
    switch (this) {
      case AppExceptionInitialize():
        break;
      default:
        break;
    }
  }
}

void setGlobalExceptionHandler() {
  FlutterError.onError = (details) {
    FlutterError.presentError(details);

    final exception = details.exception;
    final rootContext = rootNavigatorKey.currentContext;

    if (exception is AppException) {
      unawaited(exception._handle(rootContext!));
    }
  };

  PlatformDispatcher.instance.onError = (error, stack) {
    final rootContext = rootNavigatorKey.currentContext;

    if (error is AppException && rootContext != null) {
      unawaited(error._handle(rootContext));
      return true;
    }

    return false;
  };
}
