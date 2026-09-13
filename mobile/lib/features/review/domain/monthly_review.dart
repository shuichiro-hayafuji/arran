import 'review_candidate.dart';

class MonthlyReview {
  const MonthlyReview({
    required this.id,
    required this.month,
    required this.createdAt,
    required this.summary,
    required this.candidates,
  });

  final int id;
  final String month;
  final String createdAt;
  final String summary;
  final List<ReviewCandidate> candidates;
}
