import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/profile/domain/profile.dart';

part 'profile_state.freezed.dart';

@freezed
abstract class ProfileState with _$ProfileState {
  const factory ProfileState({
    required AsyncValue<Profile?> profile,
    @Default(false) bool saving,
    @Default(false) bool saved,
    Object? actionError,
  }) = _ProfileState;

  factory ProfileState.initial() => const ProfileState(profile: AsyncLoading());

  AsyncValue<Profile?> get profile;
  bool get saving;
  bool get saved;
  Object? get actionError;
}
