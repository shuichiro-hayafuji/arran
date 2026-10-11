import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:spendable_today/core/network/api_client.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import 'startup_intent.dart';
import 'startup_state.dart';

@riverpod
StartupViewModel startupViewModelFactory(Ref ref) => StartupViewModel(ref);

final startupViewModelProvider =
    StateNotifierProvider<StartupViewModel, StartupState>(
      startupViewModelFactory,
    );

/// ログイン後はフォーム入力を挟まず相談へ進む。
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
    state = state.copyWith(
      decision: const AsyncData(StartupDecision('/consult')),
    );
  }
}
