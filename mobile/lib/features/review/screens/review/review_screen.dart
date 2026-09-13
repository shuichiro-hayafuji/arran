import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:spendable_today/features/review/domain/monthly_review.dart';
import 'package:spendable_today/features/review/screens/review/review_intent.dart';
import 'package:spendable_today/features/review/screens/review/review_view_model.dart';
import 'package:spendable_today/shared/utils/formatters.dart';
import 'package:spendable_today/shared/widgets/async_error_card.dart';

class ReviewScreen extends ConsumerStatefulWidget {
  const ReviewScreen({super.key});

  @override
  ConsumerState<ReviewScreen> createState() => _ReviewScreenState();
}

class _ReviewScreenState extends ConsumerState<ReviewScreen> {
  Future<void> _run() => ref
      .read(reviewViewModelProvider.notifier)
      .dispatch(const RunMonthlyReview());

  @override
  Widget build(BuildContext context) {
    final review = ref.watch(reviewViewModelProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('今月のレビュー')),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 36),
        children: [
          Text(
            '支出を責めず、見直し候補を探す',
            style: Theme.of(
              context,
            ).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w800),
          ),
          const SizedBox(height: 8),
          const Text('金額、頻度、設定した価値観、過去の後悔を使って最大5件を抽出します。'),
          const SizedBox(height: 18),
          FilledButton.icon(
            key: const ValueKey('run-review'),
            onPressed: review.isLoading ? null : _run,
            icon: const Icon(Icons.search_outlined),
            label: const Padding(
              padding: EdgeInsets.symmetric(vertical: 13),
              child: Text('今月を分析する'),
            ),
          ),
          const SizedBox(height: 18),
          review.when(
            data: (review) => review == null
                ? const Card(
                    child: Padding(
                      padding: EdgeInsets.all(20),
                      child: Text('ボタンを押すと、現在の取引スナップショットを分析します。'),
                    ),
                  )
                : _ReviewBody(review: review),
            loading: () => const LinearProgressIndicator(),
            error: (error, _) => AsyncErrorCard(error: error, onRetry: _run),
          ),
        ],
      ),
    );
  }
}

class _ReviewBody extends StatelessWidget {
  const _ReviewBody({required this.review});

  final MonthlyReview review;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(review.summary),
        const SizedBox(height: 14),
        for (final candidate in review.candidates) ...[
          Card(
            child: Padding(
              padding: const EdgeInsets.all(18),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Chip(label: Text(candidate.judgement)),
                  const SizedBox(height: 8),
                  Text(
                    candidate.label,
                    style: Theme.of(context).textTheme.titleLarge?.copyWith(
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                  Text('${yen(candidate.totalAmount)}・${candidate.count}件'),
                  Text('年間換算: ${yen(candidate.annualizedAmount)}'),
                  const Divider(height: 24),
                  Text(candidate.reason),
                  const SizedBox(height: 10),
                  Text(
                    candidate.question,
                    style: const TextStyle(fontWeight: FontWeight.w700),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 10),
        ],
      ],
    );
  }
}
