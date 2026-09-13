import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

part 'startup_state.freezed.dart';

class StartupDecision {
  const StartupDecision(this.path);

  final String path;
}

@freezed
abstract class StartupState with _$StartupState {
  const factory StartupState({
    required AsyncValue<StartupDecision?> decision,
    required String baseUrl,
  }) = _StartupState;

  factory StartupState.initial(String baseUrl) =>
      StartupState(decision: const AsyncLoading(), baseUrl: baseUrl);

  AsyncValue<StartupDecision?> get decision;
  String get baseUrl;
}
