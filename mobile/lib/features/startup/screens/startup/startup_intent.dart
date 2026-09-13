import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/shared/presentation/mvi.dart';

part 'startup_intent.freezed.dart';

@freezed
sealed class StartupIntent with _$StartupIntent implements MviIntent {
  const StartupIntent._();

  const factory StartupIntent.initialize() = InitializeStartup;
  const factory StartupIntent.retry() = RetryStartup;
}
