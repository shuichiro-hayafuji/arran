import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/network/api_client.dart' show ApiException;

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
        content: const Text(
          '申し込み後は新しい相談と外部AIへの送信ができません。毎日日本時間0時のバッチで退会を処理します。処理できない場合は完了せず再試行します。相談履歴・取引データは保持します。',
        ),
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
    } on ApiException catch (e) {
      if (mounted && generation == authSession.generation) {
        setState(
          () => error = e.statusCode == 409
              ? '現在の状態では退会を申し込めません。処理中の相談や再有効化済みの会員についてはお問い合わせください。'
              : '申し込み結果を確認できません。状態を再確認してください。',
        );
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
              '申し込み後は新しい相談と外部AIへの送信を停止します。日本時間0時のバッチで会員を無効化します。相談履歴・取引データは保持します。障害時は完了を保留し再試行します。退会完了後の再入会・再申し込みには対応していません。お問い合わせ: spendablepartner@gmail.com',
            ),
            if (error != null && state.value?.requested != true) Text(error!),
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
              data: (status) => !status.requested && accepted
                  ? const Text('申し込みの応答を受信しましたが、現在の状態と一致しません。お問い合わせください。')
                  : status.requested
                  ? const Text('申し込みを受け付けました。退会の完了はまだ確認できません。')
                  : !status.accepting
                  ? const Text('現在、退会の申し込みを受け付けていません。お問い合わせください。')
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
