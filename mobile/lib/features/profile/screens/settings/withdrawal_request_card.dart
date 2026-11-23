import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../repository/withdrawal_request_repository.dart';
import '../../../session/provider/session_provider.dart';

class WithdrawalRequestCard extends ConsumerStatefulWidget {
  const WithdrawalRequestCard({super.key});
  @override
  ConsumerState<WithdrawalRequestCard> createState() =>
      _WithdrawalRequestCardState();
}

class _WithdrawalRequestCardState extends ConsumerState<WithdrawalRequestCard> {
  bool busy = false;
  bool accepted = false;
  String? error;

  Future<void> submit() async {
    final generation = authSession.generation;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('退会を申し込む'),
        content: const Text('申し込みを記録します。退会やデータの削除はこの時点では完了しません。削除時期は未定です。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('戻る'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('申し込む'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted || generation != authSession.generation)
      return;
    setState(() {
      busy = true;
      error = null;
    });
    try {
      await ref.read(withdrawalRequestRepositoryProvider).submit(generation);
      if (mounted && generation == authSession.generation) {
        setState(() => accepted = true);
      }
    } catch (_) {
      if (mounted && generation == authSession.generation)
        setState(() => error = '申し込み結果を確認できません。状態を再確認してください。');
    } finally {
      if (mounted && generation == authSession.generation) {
        setState(() => busy = false);
        ref.invalidate(withdrawalRequestProvider);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(withdrawalRequestProvider);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('退会', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            const Text(
              '退会を申し込むと、運営者が対応します。申し込み時点では退会・データ削除は完了せず、削除時期も未定です。お問い合わせ: spendablepartner@gmail.com',
            ),
            if (error != null && state.value != true) Text(error!),
            state.when(
              loading: () => const CircularProgressIndicator(),
              error: (_, __) => Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    accepted
                        ? '申し込みの応答を受信しましたが、現在の状態を確認できません。再読み込みしてください。'
                        : '申し込み状況を確認できません。',
                  ),
                  TextButton(
                    onPressed: () => ref.invalidate(withdrawalRequestProvider),
                    child: const Text('再読み込み'),
                  ),
                ],
              ),
              data: (requested) => !requested && accepted
                  ? const Text('申し込みの応答を受信しましたが、現在の状態と一致しません。お問い合わせください。')
                  : requested
                  ? const Text('申し込みを記録しました。運営者による確認・削除は未完了です。')
                  : FilledButton(
                      onPressed: busy ? null : submit,
                      child: const Text('退会を申し込む'),
                    ),
            ),
          ],
        ),
      ),
    );
  }
}
