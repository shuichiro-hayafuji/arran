import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:spendable_today/features/consultation/domain/consultation.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_intent.dart';
import 'package:spendable_today/features/consultation/providers/consultation_controller_provider.dart';
import 'package:spendable_today/features/consultation/providers/consultations_provider.dart';
import 'package:spendable_today/shared/utils/formatters.dart';
import 'package:spendable_today/shared/widgets/async_error_card.dart';
import 'package:spendable_today/shared/widgets/section_title.dart';

class ConsultationScreen extends ConsumerStatefulWidget {
  const ConsultationScreen({super.key});

  @override
  ConsumerState<ConsultationScreen> createState() => _ConsultationScreenState();
}

class _ConsultationScreenState extends ConsumerState<ConsultationScreen> {
  final _message = TextEditingController(text: '今から飲みに行こうと思う。行っていい？');
  final _amount = TextEditingController(text: '6000');
  final _followUp = TextEditingController();
  final _followUpAmount = TextEditingController();

  @override
  void dispose() {
    _message.dispose();
    _amount.dispose();
    _followUp.dispose();
    _followUpAmount.dispose();
    super.dispose();
  }

  void _submit() {
    if (_message.text.trim().isEmpty) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('相談内容を入力してください')));
      return;
    }
    ref
        .read(consultationControllerProvider.notifier)
        .dispatch(
          SubmitConsultation(_message.text.trim(), int.tryParse(_amount.text)),
        );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(consultationControllerProvider);
    final history = ref.watch(consultationsProvider);
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 8, 20, 36),
      children: [
        Text(
          '支出する前に相談する',
          style: Theme.of(
            context,
          ).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w800),
        ),
        const SizedBox(height: 8),
        const Text('予算だけでなく、あなたの価値観と過去の結果も判断材料にします。'),
        const SizedBox(height: 18),
        TextField(
          key: const ValueKey('consultation-message'),
          controller: _message,
          minLines: 3,
          maxLines: 6,
          decoration: const InputDecoration(
            labelText: '何に迷っていますか？',
            hintText: '例：2万円のイヤホンを買おうか迷っている',
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          key: const ValueKey('consultation-amount'),
          controller: _amount,
          keyboardType: TextInputType.number,
          inputFormatters: [FilteringTextInputFormatter.digitsOnly],
          decoration: const InputDecoration(
            labelText: '予定額（分からなければ空欄）',
            suffixText: '円',
          ),
        ),
        const SizedBox(height: 12),
        FilledButton.icon(
          key: const ValueKey('submit-consultation'),
          onPressed: state.isLoading ? null : _submit,
          icon: const Icon(Icons.psychology_outlined),
          label: const Padding(
            padding: EdgeInsets.symmetric(vertical: 14),
            child: Text('相談する'),
          ),
        ),
        if (state.isLoading) ...[
          const SizedBox(height: 14),
          const LinearProgressIndicator(),
        ],
        state.when(
          data: (consultation) => consultation == null
              ? const SizedBox.shrink()
              : Padding(
                  padding: const EdgeInsets.only(top: 20),
                  child: _AdviceCard(
                    consultation: consultation,
                    followUpController: _followUp,
                    followUpAmountController: _followUpAmount,
                    onFollowUp: () {
                      final amount =
                          int.tryParse(_followUpAmount.text) ??
                          int.tryParse(_followUp.text);
                      final answer =
                          _followUp.text.trim().isEmpty && amount != null
                          ? '予定額は$amount円です。'
                          : _followUp.text.trim();
                      ref
                          .read(consultationControllerProvider.notifier)
                          .dispatch(
                            ContinueConsultation(
                              consultation.id,
                              answer,
                              amount,
                            ),
                          );
                    },
                  ),
                ),
          loading: () => const SizedBox.shrink(),
          error: (error, _) => Padding(
            padding: const EdgeInsets.only(top: 16),
            child: AsyncErrorCard(error: error, onRetry: _submit),
          ),
        ),
        const SizedBox(height: 28),
        const SectionTitle('最近の相談'),
        const SizedBox(height: 10),
        history.when(
          loading: () => const LinearProgressIndicator(),
          error: (_, _) => const SizedBox.shrink(),
          data: (items) => items.isEmpty
              ? const Text('まだ相談履歴がありません。')
              : Card(
                  child: Column(
                    children: [
                      for (final item in items.take(5))
                        ListTile(
                          title: Text(
                            item.userMessage,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          subtitle: Text(
                            '${item.inferredCategory}・${_statusLabel(item.status)}',
                          ),
                          trailing: item.plannedAmount == null
                              ? null
                              : Text(yen(item.plannedAmount!)),
                          onTap: item.status == 'awaiting_information'
                              ? null
                              : () => context.push(
                                  '/consultations/${item.id}/result',
                                ),
                        ),
                    ],
                  ),
                ),
        ),
      ],
    );
  }
}

class _AdviceCard extends StatelessWidget {
  const _AdviceCard({
    required this.consultation,
    required this.followUpController,
    required this.followUpAmountController,
    required this.onFollowUp,
  });

  final Consultation consultation;
  final TextEditingController followUpController;
  final TextEditingController followUpAmountController;
  final VoidCallback onFollowUp;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Chip(
              avatar: const Icon(Icons.smart_toy_outlined, size: 18),
              label: Text(
                consultation.responseSource == 'openai'
                    ? 'AI回答'
                    : consultation.responseSource == 'fallback'
                    ? '定型回答（AI失敗時）'
                    : '開発用モック',
              ),
            ),
            const SizedBox(height: 12),
            const _AdviceLabel('結論'),
            Text(
              consultation.recommendation,
              key: const ValueKey('ai-recommendation'),
              style: Theme.of(
                context,
              ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800),
            ),
            const SizedBox(height: 16),
            const _AdviceLabel('判断理由'),
            Text(consultation.reasoningSummary),
            const SizedBox(height: 16),
            const _AdviceLabel('現在の状況'),
            Text(consultation.currentSituation),
            const SizedBox(height: 16),
            const _AdviceLabel('代替案'),
            Text(consultation.alternative),
            const SizedBox(height: 16),
            const _AdviceLabel('最終的な問いかけ'),
            Text(consultation.finalQuestion),
            if (consultation.needsFollowUp) ...[
              const SizedBox(height: 18),
              TextField(
                controller: followUpController,
                minLines: 2,
                maxLines: 4,
                decoration: InputDecoration(
                  labelText: consultation.followUpQuestion,
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: followUpAmountController,
                keyboardType: TextInputType.number,
                inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                decoration: const InputDecoration(
                  labelText: '予定額を更新（任意）',
                  suffixText: '円',
                ),
              ),
              const SizedBox(height: 10),
              FilledButton(onPressed: onFollowUp, child: const Text('追加情報を送る')),
            ] else ...[
              const SizedBox(height: 20),
              FilledButton.tonalIcon(
                onPressed: () =>
                    context.push('/consultations/${consultation.id}/result'),
                icon: const Icon(Icons.fact_check_outlined),
                label: const Text('実際の判断を記録する'),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _AdviceLabel extends StatelessWidget {
  const _AdviceLabel(this.text);

  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Text(
        text,
        style: TextStyle(
          color: Theme.of(context).colorScheme.primary,
          fontWeight: FontWeight.w800,
        ),
      ),
    );
  }
}

String _statusLabel(String status) => switch (status) {
  'spent' => '支出した',
  'skipped' => '見送った',
  'reduced' => '金額を減らした',
  'pending' => '保留',
  'awaiting_information' => '追加情報待ち',
  _ => '結果未記録',
};
