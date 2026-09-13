import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/consultation/domain/consultation.dart';

part 'result_state.freezed.dart';

@freezed
abstract class ResultState with _$ResultState {
  const factory ResultState({
    required AsyncValue<Consultation?> consultation,
    @Default(false) bool saving,
    @Default(false) bool saved,
    Object? actionError,
  }) = _ResultState;

  factory ResultState.initial() =>
      const ResultState(consultation: AsyncLoading());

  AsyncValue<Consultation?> get consultation;
  bool get saving;
  bool get saved;
  Object? get actionError;
}
