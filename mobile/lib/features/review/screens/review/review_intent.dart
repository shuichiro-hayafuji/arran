import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/shared/presentation/mvi.dart';

part 'review_intent.freezed.dart';

@freezed
sealed class ReviewIntent with _$ReviewIntent implements MviIntent {
  const ReviewIntent._();

  const factory ReviewIntent.runMonthly() = RunMonthlyReview;
}
