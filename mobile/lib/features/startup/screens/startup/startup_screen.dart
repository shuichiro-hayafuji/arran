import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:spendable_today/features/startup/screens/startup/startup_intent.dart';
import 'package:spendable_today/features/startup/providers/startup_provider.dart';

class StartupScreen extends ConsumerWidget {
  const StartupScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(startupViewModelProvider);
    ref.listen(startupViewModelProvider, (previous, next) {
      final path = next.decision.valueOrNull?.path;
      if (path != null && path != previous?.decision.valueOrNull?.path) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (context.mounted) context.go(path);
        });
      }
    });
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
