class ReviewCandidate {
  const ReviewCandidate({
    required this.label,
    required this.judgement,
    required this.totalAmount,
    required this.count,
    required this.reason,
    required this.annualizedAmount,
    required this.question,
  });

  final String label;
  final String judgement;
  final int totalAmount;
  final int count;
  final String reason;
  final int annualizedAmount;
  final String question;
}
