import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:spendable_today/features/startup/screens/startup/startup_intent.dart';
import 'package:spendable_today/features/startup/screens/startup/startup_view_model.dart';

class StartupScreen extends ConsumerStatefulWidget {
  const StartupScreen({super.key});

  @override
  ConsumerState<StartupScreen> createState() => _StartupScreenState();
}

class _StartupScreenState extends ConsumerState<StartupScreen> {
  @override
  void initState() {
    super.initState();
    ref.listenManual(startupViewModelProvider, (previous, next) {
      final path = next.decision.valueOrNull?.path;
      if (path != null && path != previous?.decision.valueOrNull?.path) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) return;
          if (ref.read(startupViewModelProvider).decision.valueOrNull?.path ==
              path) {
            context.go(path);
          }
        });
      }
    }, fireImmediately: true);
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(startupViewModelProvider);
    return Scaffold(
      body: Center(
        child: state.decision.when(
          data: (_) => const CircularProgressIndicator(),
          loading: () => const Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.account_balance_wallet_outlined, size: 48),
              SizedBox(height: 20),
              CircularProgressIndicator(),
              SizedBox(height: 12),
              Text('支出状況を確認しています'),
            ],
          ),
          error: (error, _) => Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.cloud_off_outlined, size: 48),
                const SizedBox(height: 12),
                Text(error.toString(), textAlign: TextAlign.center),
                const SizedBox(height: 8),
                Text(
                  '接続先: ${state.baseUrl}',
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: () => ref
                      .read(startupViewModelProvider.notifier)
                      .dispatch(const RetryStartup()),
                  child: const Text('再試行'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
