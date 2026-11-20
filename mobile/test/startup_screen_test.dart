import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:spendable_today/features/startup/screens/startup/startup_screen.dart';
import 'package:spendable_today/features/startup/screens/startup/startup_state.dart';
import 'package:spendable_today/features/startup/screens/startup/startup_view_model.dart';

void main() {
  testWidgets(
    'initial ready decision opens consultation without another update',
    (tester) async {
      final router = _router();
      addTearDown(router.dispose);
      await tester.pumpWidget(
        ProviderScope(child: MaterialApp.router(routerConfig: router)),
      );
      await tester.pump();
      await tester.pumpAndSettle();
      expect(find.text('consultation'), findsOneWidget);
      expect(find.byType(CircularProgressIndicator), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('ready update after loading navigates to consultation', (
    tester,
  ) async {
    final router = _router();
    addTearDown(router.dispose);
    late _ControlledStartupViewModel model;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          startupViewModelProvider.overrideWith((ref) {
            model = _ControlledStartupViewModel(ref);
            return model;
          }),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    model.ready();
    await tester.pump();
    await tester.pump();
    await tester.pumpAndSettle();
    expect(find.text('consultation'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('queued decision cannot navigate after startup is removed', (
    tester,
  ) async {
    final router = _router();
    addTearDown(router.dispose);
    late _ControlledStartupViewModel model;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          startupViewModelProvider.overrideWith((ref) {
            model = _ControlledStartupViewModel(ref);
            return model;
          }),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    model.ready();
    await tester.pumpWidget(const SizedBox.shrink());
    expect(router.routeInformationProvider.value.uri.path, '/');
    expect(tester.takeException(), isNull);
  });
}

GoRouter _router() => GoRouter(
  routes: [
    GoRoute(path: '/', builder: (_, _) => const StartupScreen()),
    GoRoute(
      path: '/consult',
      builder: (_, _) => const Scaffold(body: Text('consultation')),
    ),
  ],
);

class _ControlledStartupViewModel extends StartupViewModel {
  _ControlledStartupViewModel(super.ref) {
    state = state.copyWith(decision: const AsyncLoading());
  }

  void ready() {
    state = state.copyWith(
      decision: const AsyncData(StartupDecision('/consult')),
    );
  }
}
