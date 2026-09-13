import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../screens/result/result_state.dart';
import '../screens/result/result_view_model.dart';

@riverpod
ResultViewModel resultViewModelFactory(Ref ref, int consultationId) =>
    ResultViewModel(ref, consultationId);

final resultViewModelProvider =
    StateNotifierProvider.family<ResultViewModel, ResultState, int>(
      resultViewModelFactory,
    );
