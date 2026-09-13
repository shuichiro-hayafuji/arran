import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../features/consultation/screens/consultation/consultation_screen.dart';
import '../../features/consultation/screens/result/result_screen.dart';
import '../../features/dashboard/screens/dashboard/dashboard_screen.dart';
import '../../features/profile/screens/memories/memory_screen.dart';
import '../../features/profile/screens/profile/profile_screen.dart';
import '../../features/profile/screens/settings/settings_screen.dart';
import '../../features/review/screens/review/review_screen.dart';
import '../../features/startup/screens/startup/startup_screen.dart';
import '../../features/transactions/screens/import/import_screen.dart';
import '../../features/transactions/screens/detail/transaction_detail_screen.dart';
import '../../features/transactions/screens/list/transactions_screen.dart';
import 'navigation_shell.dart';
import '../../features/session/provider/session_provider.dart';
import '../../features/login/screens/login/login_screen.dart';

final rootNavigatorKey = GlobalKey<NavigatorState>(
  debugLabel: 'rootNavigatorKey',
);

final appRouter = GoRouter(
  navigatorKey: rootNavigatorKey,
  refreshListenable: authSession,
  redirect: (context, state) {
    if (!authSession.initialized || !authSession.loggedIn) {
      return state.matchedLocation == '/login' ? null : '/login';
    }
    return state.matchedLocation == '/login' ? '/' : null;
  },
  initialLocation: '/',
  routes: [
    GoRoute(path: '/login', builder: (context, state) => const LoginScreen()),
    GoRoute(path: '/', builder: (context, state) => const StartupScreen()),
    ShellRoute(
      builder: (context, state, child) =>
          NavigationShell(state: state, child: child),
      routes: [
        GoRoute(
          path: '/home',
          builder: (context, state) => const DashboardScreen(),
        ),
        GoRoute(
          path: '/transactions',
          builder: (context, state) => const TransactionsScreen(),
        ),
        GoRoute(
          path: '/consult',
          builder: (context, state) => const ConsultationScreen(),
        ),
        GoRoute(
          path: '/settings',
          builder: (context, state) => const SettingsScreen(),
        ),
      ],
    ),
    GoRoute(path: '/import', builder: (context, state) => const ImportScreen()),
    GoRoute(
      path: '/transactions/:id',
      builder: (context, state) => TransactionDetailScreen(
        transactionId: int.parse(state.pathParameters['id']!),
      ),
    ),
    GoRoute(
      path: '/consultations/:id/result',
      builder: (context, state) =>
          ResultScreen(consultationId: int.parse(state.pathParameters['id']!)),
    ),
    GoRoute(path: '/review', builder: (context, state) => const ReviewScreen()),
    GoRoute(
      path: '/profile',
      builder: (context, state) => ProfileScreen(
        onboarding: state.uri.queryParameters['onboarding'] == 'true',
      ),
    ),
    GoRoute(
      path: '/memories',
      builder: (context, state) => const MemoryScreen(),
    ),
  ],
  errorBuilder: (context, state) => Scaffold(
    appBar: AppBar(title: const Text('ページが見つかりません')),
    body: Center(child: Text(state.error.toString())),
  ),
);
