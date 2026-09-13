import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../repository/review_repository.dart';

@riverpod
ReviewRepository reviewRepository(Ref ref) =>
    ReviewRepositoryImpl(ref.watch(authorizedApiClientProvider));

final reviewRepositoryProvider = Provider<ReviewRepository>(reviewRepository);
