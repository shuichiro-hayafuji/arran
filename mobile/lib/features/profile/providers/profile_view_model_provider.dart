import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../screens/profile/profile_state.dart';
import '../screens/profile/profile_view_model.dart';

@riverpod
ProfileViewModel profileViewModelFactory(Ref ref, bool onboarding) =>
    ProfileViewModel(ref, onboarding);

final profileViewModelProvider =
    StateNotifierProvider.family<ProfileViewModel, ProfileState, bool>(
      profileViewModelFactory,
    );
