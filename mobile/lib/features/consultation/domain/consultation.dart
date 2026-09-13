class Consultation {
  const Consultation({
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
}
