import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../domain/profile.dart';
import 'profile_repository_provider.dart';

@riverpod
Future<Profile> profile(Ref ref) => ref.read(profileRepositoryProvider).get();

final profileProvider = FutureProvider<Profile>(profile);
