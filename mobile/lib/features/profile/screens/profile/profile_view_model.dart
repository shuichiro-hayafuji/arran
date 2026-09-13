import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:spendable_today/features/profile/domain/profile.dart';
import 'package:spendable_today/features/profile/repository/profile_repository.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import 'profile_intent.dart';
import 'profile_state.dart';

@riverpod
ProfileViewModel profileViewModelFactory(Ref ref, bool onboarding) =>
    ProfileViewModel(ref, onboarding);

final profileViewModelProvider =
    StateNotifierProvider.family<ProfileViewModel, ProfileState, bool>(
      profileViewModelFactory,
    );

/// 初回登録では取得を省略し、通常編集では保存済みプロフィールを読み込む。
/// 保存失敗は actionError に分け、読み込み済みのプロフィールを維持する。
class ProfileViewModel extends MviViewModel<ProfileState, ProfileIntent> {
  ProfileViewModel(this.ref, this.onboarding) : super(ProfileState.initial()) {
    dispatch(const LoadProfile());
  }

  final Ref ref;
  final bool onboarding;

  @override
  Future<void> dispatch(ProfileIntent intent) {
    switch (intent) {
      case LoadProfile():
        return _load();
      case SaveProfile(profile: final profile):
        return _save(profile);
    }
  }

  Future<void> _load() async {
    if (onboarding) {
      state = state.copyWith(profile: const AsyncData(null));
      return;
    }
    try {
      final loaded = await ref.read(profileRepositoryProvider).get();
      if (!mounted) return;
      state = state.copyWith(profile: AsyncData(loaded));
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = state.copyWith(profile: AsyncError(error, stackTrace));
    }
  }

  Future<void> _save(Profile profile) async {
    state = state.copyWith(saving: true, saved: false, actionError: null);
    try {
      final savedProfile = await ref
          .read(profileRepositoryProvider)
          .save(profile);
      if (!mounted) return;
      state = state.copyWith(
        profile: AsyncData(savedProfile),
        saving: false,
        saved: true,
      );
    } catch (error) {
      if (!mounted) return;
      state = state.copyWith(saving: false, actionError: error);
    }
  }
}
