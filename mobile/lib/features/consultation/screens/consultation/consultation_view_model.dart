import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:spendable_today/shared/presentation/mvi.dart';
import 'package:spendable_today/features/consultation/domain/consultation.dart';
import 'package:spendable_today/features/consultation/providers/consultation_repository_provider.dart';
import 'package:spendable_today/features/consultation/providers/consultations_provider.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_intent.dart';

/// 新規相談と追加回答を送信し、サーバーの判定結果を画面と履歴に反映する。
/// 予定額の未入力は null のまま送り、追加質問の要否はサーバーに委ねる。
class ConsultationViewModel
    extends MviViewModel<AsyncValue<Consultation?>, ConsultationIntent> {
  ConsultationViewModel(this.ref) : super(const AsyncData(null));

  final Ref ref;

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
      case ClearConsultation():
        clear();
        return Future.value();
    }
  }

  Future<void> submit(String message, int? plannedAmount) async {
    state = const AsyncLoading();
    try {
      final consultation = await ref
          .read(consultationRepositoryProvider)
          .start(message, plannedAmount);
      if (!mounted) return;
      state = AsyncData(consultation);
      ref.invalidate(consultationsProvider);
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = AsyncError(error, stackTrace);
    }
  }

  Future<void> continueWith(int id, String message, int? plannedAmount) async {
    state = const AsyncLoading();
    try {
      final consultation = await ref
          .read(consultationRepositoryProvider)
          .continueConversation(id, message, plannedAmount);
      if (!mounted) return;
      state = AsyncData(consultation);
      ref.invalidate(consultationsProvider);
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = AsyncError(error, stackTrace);
    }
  }

  void clear() => state = const AsyncData(null);
}
