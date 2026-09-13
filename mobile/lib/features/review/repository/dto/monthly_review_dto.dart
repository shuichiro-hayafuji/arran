import '../../../../shared/utils/json_parsing.dart';
import '../../domain/monthly_review.dart';
import '../../domain/review_candidate.dart';

class ReviewCandidateDto {
  const ReviewCandidateDto({
    required this.label,
    required this.judgement,
    required this.totalAmount,
    required this.count,
    required this.reason,
    required this.annualizedAmount,
    required this.question,
  });

  factory ReviewCandidateDto.fromJson(Map<String, dynamic> json) =>
      ReviewCandidateDto(
        label: json['label'] as String? ?? '',
        judgement: json['judgement'] as String? ?? '',
        totalAmount: parseInt(json['total_amount']),
        count: parseInt(json['count']),
        reason: json['reason'] as String? ?? '',
        annualizedAmount: parseInt(json['annualized_amount']),
        question: json['question'] as String? ?? '',
      );

  final String label;
  final String judgement;
  final int totalAmount;
  final int count;
  final String reason;
  final int annualizedAmount;
  final String question;

  ReviewCandidate toDomain() => ReviewCandidate(
    label: label,
    judgement: judgement,
    totalAmount: totalAmount,
    count: count,
    reason: reason,
    annualizedAmount: annualizedAmount,
    question: question,
  );
}

class MonthlyReviewDto {
  const MonthlyReviewDto({
    required this.id,
    required this.month,
    required this.createdAt,
    required this.summary,
    required this.candidates,
  });

  factory MonthlyReviewDto.fromJson(Map<String, dynamic> json) =>
      MonthlyReviewDto(
        id: parseInt(json['id']),
        month: json['month'] as String? ?? '',
        createdAt: json['created_at'] as String? ?? '',
        summary: json['summary'] as String? ?? '',
        candidates: parseMaps(
          json['candidates'],
        ).map(ReviewCandidateDto.fromJson).toList(),
      );

  final int id;
  final String month;
  final String createdAt;
  final String summary;
  final List<ReviewCandidateDto> candidates;

  MonthlyReview toDomain() => MonthlyReview(
    id: id,
    month: month,
    createdAt: createdAt,
    summary: summary,
    candidates: candidates.map((dto) => dto.toDomain()).toList(),
  );
}
