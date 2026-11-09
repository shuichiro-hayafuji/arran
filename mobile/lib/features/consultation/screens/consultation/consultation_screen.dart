import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/constants/space_theme.dart';
import '../../../../shared/widgets/async_error_card.dart';
import 'consultation_intent.dart';
import 'consultation_view_model.dart';

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
    if (!mounted) return;
    final saved = ref
        .read(consultationControllerProvider)
        .consultation
        .valueOrNull;
    if (saved != null &&
        !ref.read(consultationControllerProvider).consultation.hasError &&
        !ref.read(consultationControllerProvider).consultation.isLoading) {
      _message.clear();
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(consultationControllerProvider);
    final item = state.consultation.valueOrNull;
    return ListView(
      padding: AppSpace.pXl,
      children: [
        Text('支出について相談する', style: Theme.of(context).textTheme.headlineSmall),
        AppSpace.syM,
        const Text('分かる範囲で話してください。金額の入力やCSVの取込は必須ではありません。'),
        AppSpace.syXl,
        if (item != null) ...[
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
          enabled: !state.consultation.isLoading,
          decoration: const InputDecoration(
            labelText: '相談・補足・訂正',
            hintText: '例：2万円のイヤホンを買おうか迷っている',
          ),
        ),
        AppSpace.syL,
        FilledButton(
          key: const ValueKey('submit-consultation'),
          onPressed: state.consultation.isLoading ? null : _send,
          child: const Text('送る'),
        ),
        if (state.consultation.isLoading) ...[
          AppSpace.syM,
          const LinearProgressIndicator(),
        ],
        if (state.consultation.hasError)
          AsyncErrorCard(error: state.consultation.error!, onRetry: _send),
        AppSpace.syM,
        TextButton(
          onPressed: state.consultation.isLoading
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
                  onTap: state.consultation.isLoading
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
