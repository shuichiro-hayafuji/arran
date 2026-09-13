import 'dart:async';
import 'package:flutter/material.dart';
import 'features/session/provider/session_provider.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:spendable_today/app/exception/global_exception_handler.dart';
import 'package:spendable_today/app/constants/app_theme.dart';

import 'app/router/router.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  setGlobalExceptionHandler();
  unawaited(authSession.initialize());
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: authSession,
      builder: (context, _) => ProviderScope(
        // ログインの切替・失効時に、前の利用者のデータを含む Provider を作り直す。
        key: ValueKey(authSession.generation),
        child: MaterialApp.router(
          title: 'AI 家計アドバイザー',
          routerConfig: appRouter,
          theme: AppTheme.standard,
        ),
      ),
    );
  }
}
