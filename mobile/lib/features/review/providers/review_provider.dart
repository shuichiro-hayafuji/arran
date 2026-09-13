import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../screens/review/review_view_model.dart';
import '../domain/monthly_review.dart';

@riverpod
ReviewViewModel reviewViewModelFactory(Ref ref) => ReviewViewModel(ref);

final reviewViewModelProvider =
    StateNotifierProvider<ReviewViewModel, AsyncValue<MonthlyReview?>>(
      reviewViewModelFactory,
    );
