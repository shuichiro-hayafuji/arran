import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';
import '../domain/profile.dart';
import 'dto/profile_dto.dart';

@riverpod
ProfileRepository profileRepository(Ref ref) =>
    ProfileRepositoryImpl(ref.watch(authorizedApiClientProvider));

final profileRepositoryProvider = Provider<ProfileRepository>(
  profileRepository,
);

abstract interface class ProfileRepository {
  Future<Profile> get();
  Future<Profile> save(Profile profile);
}

class ProfileRepositoryImpl implements ProfileRepository {
  const ProfileRepositoryImpl(this._client);

  final ApiClient _client;

  @override
  Future<Profile> get() async {
    final response = await _client.get('/profile');
    return ProfileDto.fromJson(parseMap(response.data)).toDomain();
  }

  @override
  Future<Profile> save(Profile profile) async {
    final response = await _client.put(
      '/profile',
      data: ProfileDto.fromDomain(profile).toJson(),
    );
    return ProfileDto.fromJson(parseMap(response.data)).toDomain();
  }
}
