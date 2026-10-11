import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:spendable_today/shared/presentation/mvi.dart';
import 'package:spendable_today/features/consultation/repository/consultation_repository.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_intent.dart';

import 'consultation_state.dart';
import '../../domain/consultation.dart';

@riverpod
ConsultationViewModel consultationViewModelFactory(Ref ref) =>
    ConsultationViewModel(ref);

final consultationControllerProvider =
    StateNotifierProvider<ConsultationViewModel, ConsultationState>(
      consultationViewModelFactory,
    );

/// 新規相談と追加回答を送信し、サーバーの判定結果を画面と履歴に反映する。
/// 予定額の未入力は null のまま送り、追加質問の要否はサーバーに委ねる。
class ConsultationViewModel
    extends MviViewModel<ConsultationState, ConsultationIntent> {
  ConsultationViewModel(this.ref) : super(const ConsultationState()) {
    unawaited(refreshHistory());
  }

  final Ref ref;
  int _historyRequestId = 0;

  @override
  Future<void> dispatch(ConsultationIntent intent) {
    switch (intent) {
      case SubmitConsultation(
        message: final message,
        plannedAmount: final amount,
      ):
        return submit(message, amount);
      case ContinueConsultation(
        id: final id,
        message: final message,
        plannedAmount: final amount,
      ):
        return continueWith(id, message, amount);
      case SelectConsultation(consultation: final item):
        select(item);
        return Future.value();
      case ClearConsultation():
        clear();
        return Future.value();
    }
  }

  Future<void> submit(String message, int? plannedAmount) async {
    if (state.consultation.isLoading) return;
    final previous = state.consultation;
    state = state.copyWith(
      consultation: const AsyncLoading<Consultation?>().copyWithPrevious(
        previous,
      ),
    );
    try {
      final consultation = await ref
          .read(consultationRepositoryProvider)
          .start(message, plannedAmount);
      if (!mounted) return;
      await refreshHistory();
      if (!mounted) return;
      state = state.copyWith(consultation: AsyncData(consultation));
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = state.copyWith(
        consultation: AsyncError<Consultation?>(
          error,
          stackTrace,
        ).copyWithPrevious(previous),
      );
    }
  }

  Future<void> continueWith(int id, String message, int? plannedAmount) async {
    if (state.consultation.isLoading) return;
    final previous = state.consultation;
    state = state.copyWith(
      consultation: const AsyncLoading<Consultation?>().copyWithPrevious(
        previous,
      ),
    );
    try {
      final consultation = await ref
          .read(consultationRepositoryProvider)
          .continueConversation(id, message, plannedAmount);
      if (!mounted) return;
      await refreshHistory();
      if (!mounted) return;
      state = state.copyWith(consultation: AsyncData(consultation));
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = state.copyWith(
        consultation: AsyncError<Consultation?>(
          error,
          stackTrace,
        ).copyWithPrevious(previous),
      );
    }
  }

  /// 初期表示と相談・結果の保存後に履歴を再取得する。
  /// 遅れて返った古いリクエストで最新の履歴を上書きしない。
  Future<void> refreshHistory() async {
    final requestId = ++_historyRequestId;
    state = state.copyWith(history: const AsyncLoading());
    try {
      final items = await ref.read(consultationRepositoryProvider).getAll();
      if (!mounted || requestId != _historyRequestId) return;
      state = state.copyWith(history: AsyncData(List.unmodifiable(items)));
    } catch (error, stackTrace) {
      if (!mounted || requestId != _historyRequestId) return;
      state = state.copyWith(history: AsyncError(error, stackTrace));
    }
  }

  void select(Consultation item) {
    if (state.consultation.isLoading) return;
    state = state.copyWith(consultation: AsyncData(item));
  }

  void clear() {
    if (state.consultation.isLoading) return;
    state = state.copyWith(consultation: const AsyncData(null));
  }
}
