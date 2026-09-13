import '../../../../shared/utils/json_parsing.dart';
import '../../domain/consultation.dart';

class ConsultationDto {
  const ConsultationDto({
    required this.id,
    required this.createdAt,
    required this.userMessage,
    required this.plannedAmount,
    required this.inferredCategory,
    required this.recommendation,
    required this.reasoningSummary,
    required this.currentSituation,
    required this.alternative,
    required this.finalQuestion,
    required this.status,
    required this.actualAmount,
    required this.userDecisionReason,
    required this.satisfactionScore,
    required this.regretScore,
    required this.note,
    required this.responseSource,
    required this.needsFollowUp,
    required this.followUpQuestion,
  });

  factory ConsultationDto.fromJson(Map<String, dynamic> json) =>
      ConsultationDto(
        id: parseInt(json['id']),
        createdAt: json['created_at'] as String? ?? '',
        userMessage: json['user_message'] as String? ?? '',
        plannedAmount: parseNullableInt(json['planned_amount']),
        inferredCategory: json['inferred_category'] as String? ?? '',
        recommendation: json['ai_recommendation'] as String? ?? '',
        reasoningSummary: json['ai_reasoning_summary'] as String? ?? '',
        currentSituation: json['current_situation'] as String? ?? '',
        alternative: json['alternative'] as String? ?? '',
        finalQuestion: json['final_question'] as String? ?? '',
        status: json['status'] as String? ?? '',
        actualAmount: parseNullableInt(json['actual_amount']),
        userDecisionReason: json['user_decision_reason'] as String? ?? '',
        satisfactionScore: parseNullableInt(json['satisfaction_score']),
        regretScore: parseNullableInt(json['regret_score']),
        note: json['note'] as String? ?? '',
        responseSource: json['response_source'] as String? ?? '',
        needsFollowUp: json['needs_follow_up'] as bool? ?? false,
        followUpQuestion: json['follow_up_question'] as String? ?? '',
      );

  final int id;
  final String createdAt;
  final String userMessage;
  final int? plannedAmount;
  final String inferredCategory;
  final String recommendation;
  final String reasoningSummary;
  final String currentSituation;
  final String alternative;
  final String finalQuestion;
  final String status;
  final int? actualAmount;
  final String userDecisionReason;
  final int? satisfactionScore;
  final int? regretScore;
  final String note;
  final String responseSource;
  final bool needsFollowUp;
  final String followUpQuestion;

  Consultation toDomain() => Consultation(
    id: id,
    createdAt: createdAt,
    userMessage: userMessage,
    plannedAmount: plannedAmount,
    inferredCategory: inferredCategory,
    recommendation: recommendation,
    reasoningSummary: reasoningSummary,
    currentSituation: currentSituation,
    alternative: alternative,
    finalQuestion: finalQuestion,
    status: status,
    actualAmount: actualAmount,
    userDecisionReason: userDecisionReason,
    satisfactionScore: satisfactionScore,
    regretScore: regretScore,
    note: note,
    responseSource: responseSource,
    needsFollowUp: needsFollowUp,
    followUpQuestion: followUpQuestion,
  );
}

class StartConsultationRequestDto {
  const StartConsultationRequestDto(this.message, this.plannedAmount);

  final String message;
  final int? plannedAmount;

  Map<String, dynamic> toJson() => {
    'message': message,
    'planned_amount': plannedAmount,
  };
}

class ContinueConsultationRequestDto extends StartConsultationRequestDto {
  const ContinueConsultationRequestDto(super.message, super.plannedAmount);
}

class UpdateConsultationResultRequestDto {
  const UpdateConsultationResultRequestDto({
    required this.status,
    required this.actualAmount,
    required this.reason,
    required this.satisfaction,
    required this.regret,
    required this.note,
  });

  final String status;
  final int? actualAmount;
  final String reason;
  final int? satisfaction;
  final int? regret;
  final String note;

  Map<String, dynamic> toJson() => {
    'status': status,
    'actual_amount': actualAmount,
    'user_decision_reason': reason,
    'satisfaction_score': satisfaction,
    'regret_score': regret,
    'note': note,
  };
}
