import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../repository/profile_repository.dart';

@riverpod
ProfileRepository profileRepository(Ref ref) =>
    ProfileRepositoryImpl(ref.watch(authorizedApiClientProvider));

final profileRepositoryProvider = Provider<ProfileRepository>(
  profileRepository,
);
