import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:spendable_today/features/review/domain/monthly_review.dart';
import 'package:spendable_today/features/review/providers/review_repository_provider.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import 'review_intent.dart';

/// 利用者の実行操作で月次レビューを生成する。null は未実行の状態を表す。
class ReviewViewModel
    extends MviViewModel<AsyncValue<MonthlyReview?>, ReviewIntent> {
  ReviewViewModel(this.ref) : super(const AsyncData(null));

  final Ref ref;

  @override
  Future<void> dispatch(ReviewIntent intent) {
    switch (intent) {
      case RunMonthlyReview():
        return _run();
    }
  }

  Future<void> _run() async {
    state = const AsyncLoading();
    try {
      final loaded = await ref.read(reviewRepositoryProvider).createMonthly();
      if (!mounted) return;
      state = AsyncData(loaded);
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = AsyncError(error, stackTrace);
    }
  }
}
