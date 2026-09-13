import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:spendable_today/core/network/api_client.dart';
import 'package:spendable_today/features/profile/providers/profile_repository_provider.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import 'startup_intent.dart';
import 'startup_state.dart';

/// プロフィールの有無で初期遷移先を決める。
/// 404 だけを初回登録と扱い、通信障害などは再試行できるエラーとして残す。
class StartupViewModel extends MviViewModel<StartupState, StartupIntent> {
  StartupViewModel(this.ref)
    : super(StartupState.initial(ref.read(publicApiClientProvider).baseUrl)) {
    dispatch(const InitializeStartup());
  }

  final Ref ref;

  @override
  Future<void> dispatch(StartupIntent intent) {
    switch (intent) {
      case InitializeStartup():
      case RetryStartup():
        return _load();
    }
  }

  Future<void> _load() async {
    state = state.copyWith(decision: const AsyncLoading());
    try {
      await ref.read(profileRepositoryProvider).get();
      if (!mounted) return;
      state = state.copyWith(
        decision: const AsyncData(StartupDecision('/home')),
      );
    } catch (error, stackTrace) {
      if (!mounted) return;
      if (error is ApiException && error.statusCode == 404) {
        state = state.copyWith(
          decision: const AsyncData(
            StartupDecision('/profile?onboarding=true'),
          ),
        );
      } else {
        state = state.copyWith(decision: AsyncError(error, stackTrace));
      }
    }
  }
}
