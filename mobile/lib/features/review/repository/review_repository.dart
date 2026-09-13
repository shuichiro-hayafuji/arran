import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';
import '../domain/monthly_review.dart';
import 'dto/monthly_review_dto.dart';

@riverpod
ReviewRepository reviewRepository(Ref ref) =>
    ReviewRepositoryImpl(ref.watch(authorizedApiClientProvider));

final reviewRepositoryProvider = Provider<ReviewRepository>(reviewRepository);

abstract interface class ReviewRepository {
  Future<MonthlyReview> createMonthly();
}

class ReviewRepositoryImpl implements ReviewRepository {
  const ReviewRepositoryImpl(this._client);

  final ApiClient _client;

  @override
  Future<MonthlyReview> createMonthly() async {
    final response = await _client.post(
      '/reviews/monthly',
      data: <String, dynamic>{},
    );
    return MonthlyReviewDto.fromJson(parseMap(response.data)).toDomain();
  }
}
