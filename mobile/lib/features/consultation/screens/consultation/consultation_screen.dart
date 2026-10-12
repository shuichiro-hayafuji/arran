import '../../../transactions/screens/import/purchase_csv_confirmation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/constants/space_theme.dart';
import '../../../../shared/widgets/async_error_card.dart';
import 'consultation_intent.dart';
import 'consultation_view_model.dart';
import 'consultation_state.dart';

class ConsultationScreen extends ConsumerStatefulWidget {
  const ConsultationScreen({super.key});
  @override
  ConsumerState<ConsultationScreen> createState() => _ConsultationScreenState();
}

class _ConsultationScreenState extends ConsumerState<ConsultationScreen> {
  final _message = TextEditingController();
  @override
  void dispose() {
    _message.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    if (ref.read(consultationControllerProvider).unresolved) {
      await ref
          .read(consultationControllerProvider.notifier)
          .dispatch(const RecoverConsultationRequest());
      return;
    }
    final text = _message.text.trim();
    if (text.isEmpty) return;
    final vm = ref.read(consultationControllerProvider.notifier);
    final active = ref
        .read(consultationControllerProvider)
        .consultation
        .valueOrNull;
    final threadId = active != null && active.messages.isNotEmpty
        ? active.id
        : null;
    await vm.dispatch(
      threadId == null
          ? SubmitConsultation(text, null)
          : ContinueConsultation(threadId, text, null),
    );
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(consultationControllerProvider, (previous, next) {
      if (previous?.unresolved == true &&
          !next.unresolved &&
          next.consultation.valueOrNull != null &&
          !next.consultation.hasError &&
          !next.consultation.isLoading) {
        _message.clear();
      }
      if (_message.text.isEmpty && next.pendingRequest != null) {
        _message.text = next.pendingRequest!.message;
      }
    });
    final state = ref.watch(consultationControllerProvider);
    if (_message.text.isEmpty && state.pendingRequest != null) {
      _message.text = state.pendingRequest!.message;
    }
    final item = state.consultation.valueOrNull;
    return ListView(
      padding: AppSpace.pXl,
      children: [
        Text('支出について相談する', style: Theme.of(context).textTheme.headlineSmall),
        AppSpace.syM,
        const Text('分かる範囲で話してください。金額の入力やCSVの取込は必須ではありません。'),
        AppSpace.syXl,
        if (item != null) ...[
          for (final p in item.purchases.where((p) => p.csvPending))
            PurchaseCsvConfirmation(
              key: ValueKey(p.id),
              purchaseId: p.id,
              description: p.description,
            ),
          if (item.messages.isEmpty) Text(item.recommendation),
          for (final message in item.messages)
            Card(
              child: Padding(
                padding: AppSpace.pXl,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      message.role == 'user' ? 'あなた' : 'Arran',
                      style: Theme.of(context).textTheme.labelLarge,
                    ),
                    AppSpace.syS,
                    SelectableText(message.content),
                    if (message.reason.isNotEmpty) Text(message.reason),
                    if (message.assumptions.isNotEmpty)
                      Text(message.assumptions),
                  ],
                ),
              ),
            ),
          if (item.messages.isEmpty && item.reasoningSummary.isNotEmpty)
            Text(item.reasoningSummary),
          if (item.messages.isEmpty && item.currentSituation.isNotEmpty)
            Text(item.currentSituation),
          if (item.status == 'unavailable')
            const Text('相談内容を残しています。下の入力欄から「もう一度答えて」と送れます。'),
          if (item.messages.any(
            (message) =>
                message.role == 'assistant' &&
                message.content.contains('任意でCSV明細'),
          ))
            TextButton(
              onPressed: () => context.push('/import'),
              child: const Text('CSV取込を開く'),
            ),
          AppSpace.syXl,
        ],
        TextField(
          key: const ValueKey('consultation-message'),
          controller: _message,
          minLines: 2,
          maxLines: 6,
          enabled: !state.consultation.isLoading && !state.unresolved,
          decoration: const InputDecoration(
            labelText: '相談・補足・訂正',
            hintText: '例：2万円のイヤホンを買おうか迷っている',
          ),
        ),
        AppSpace.syL,
        FilledButton(
          key: const ValueKey('submit-consultation'),
          onPressed: state.consultation.isLoading ? null : _send,
          child: Text(state.unresolved ? '結果を確認' : '送る'),
        ),
        if (state.consultation.isLoading) ...[
          AppSpace.syM,
          const LinearProgressIndicator(),
          Text(
            state.phase == ConsultationPhase.cancelling
                ? '中止を確認しています。保存が完了している場合は結果を表示します。'
                : '回答を作成しています。通信が切れても同じ送信の保存結果を確認します。',
          ),
        ],
        if (state.unresolved)
          TextButton(
            onPressed: () => ref
                .read(consultationControllerProvider.notifier)
                .dispatch(const CancelConsultationRequest()),
            child: const Text('処理の中止を依頼'),
          ),
        if (state.consultation.hasError)
          AsyncErrorCard(error: state.consultation.error!, onRetry: _send),
        AppSpace.syM,
        TextButton(
          onPressed: (state.consultation.isLoading || state.unresolved)
              ? null
              : () {
                  _message.clear();
                  ref
                      .read(consultationControllerProvider.notifier)
                      .dispatch(const ClearConsultation());
                },
          child: const Text('新しい相談を始める'),
        ),
        AppSpace.syXxl,
        Text('最近の相談', style: Theme.of(context).textTheme.titleMedium),
        state.history.when(
          data: (items) => Column(
            children: [
              if (items.isEmpty) const Text('まだ相談履歴がありません。'),
              for (final entry in items)
                ListTile(
                  title: Text(
                    entry.userMessage,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  subtitle: Text(
                    entry.recommendation,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  onTap: (state.consultation.isLoading || state.unresolved)
                      ? null
                      : () {
                          ref
                              .read(consultationControllerProvider.notifier)
                              .dispatch(SelectConsultation(entry));
                        },
                ),
            ],
          ),
          loading: () => const LinearProgressIndicator(),
          error: (error, _) => AsyncErrorCard(
            error: error,
            onRetry: () => ref
                .read(consultationControllerProvider.notifier)
                .refreshHistory(),
          ),
        ),
      ],
    );
  }
}
