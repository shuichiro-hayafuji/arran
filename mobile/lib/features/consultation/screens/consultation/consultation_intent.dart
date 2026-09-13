import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/shared/presentation/mvi.dart';

part 'consultation_intent.freezed.dart';

@freezed
sealed class ConsultationIntent with _$ConsultationIntent implements MviIntent {
  const ConsultationIntent._();

  const factory ConsultationIntent.submit(String message, int? plannedAmount) =
      SubmitConsultation;
  const factory ConsultationIntent.continueWith(
    int id,
    String message,
    int? plannedAmount,
  ) = ContinueConsultation;
  const factory ConsultationIntent.clear() = ClearConsultation;
}
