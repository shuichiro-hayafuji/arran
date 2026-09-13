import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:spendable_today/features/consultation/repository/consultation_repository.dart';
import 'package:spendable_today/features/profile/screens/memories/memory_view_model.dart';
import 'package:spendable_today/features/profile/screens/memories/memory_intent.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import '../consultation/consultation_view_model.dart';
import 'result_intent.dart';
import 'result_state.dart';

@riverpod
ResultViewModel resultViewModelFactory(Ref ref, int consultationId) =>
    ResultViewModel(ref, consultationId);

final resultViewModelProvider =
    StateNotifierProvider.family<ResultViewModel, ResultState, int>(
      resultViewModelFactory,
    );

class ResultViewModel extends MviViewModel<ResultState, ResultIntent> {
  ResultViewModel(this.ref, this.consultationId)
    : super(ResultState.initial()) {
    dispatch(const LoadResult());
  }

  final Ref ref;
  final int consultationId;

  @override
  Future<void> dispatch(ResultIntent intent) {
    switch (intent) {
      case LoadResult():
        return _load();
      case SaveResult(
        status: final status,
        actualAmount: final actualAmount,
        reason: final reason,
        satisfaction: final satisfaction,
        regret: final regret,
        note: final note,
      ):
        return _save(
          status: status,
          actualAmount: actualAmount,
          reason: reason,
          satisfaction: satisfaction,
          regret: regret,
          note: note,
        );
    }
  }

  Future<void> _load() async {
    state = state.copyWith(consultation: const AsyncLoading());
    try {
      final items = await ref.read(consultationRepositoryProvider).getAll();
      if (!mounted) return;
      state = state.copyWith(
        consultation: AsyncData(
          items.where((item) => item.id == consultationId).firstOrNull,
        ),
      );
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = state.copyWith(consultation: AsyncError(error, stackTrace));
    }
  }

  Future<void> _save({
    required String status,
    required int? actualAmount,
    required String reason,
    required int satisfaction,
    required int regret,
    required String note,
  }) async {
    state = state.copyWith(saving: true, saved: false, actionError: null);
    try {
      await ref
          .read(consultationRepositoryProvider)
          .updateResult(
            consultationId,
            status: status,
            actualAmount: actualAmount,
            reason: reason,
            satisfaction: satisfaction,
            regret: regret,
            note: note,
          );
      if (!mounted) return;
      if (ref.exists(consultationControllerProvider)) {
        unawaited(
          ref.read(consultationControllerProvider.notifier).refreshHistory(),
        );
      }
      ref.read(memoriesViewModelProvider.notifier).dispatch(RefreshMemories());
      state = state.copyWith(saving: false, saved: true);
    } catch (error) {
      if (!mounted) return;
      state = state.copyWith(saving: false, actionError: error);
    }
  }
}
