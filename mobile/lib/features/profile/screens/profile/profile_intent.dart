import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/profile/domain/profile.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';

part 'profile_intent.freezed.dart';

@freezed
sealed class ProfileIntent with _$ProfileIntent implements MviIntent {
  const ProfileIntent._();

  const factory ProfileIntent.load() = LoadProfile;
  const factory ProfileIntent.save(Profile profile) = SaveProfile;
}
