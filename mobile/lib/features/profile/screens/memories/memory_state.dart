import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/profile/domain/memory_item.dart';

part 'memory_state.freezed.dart';

@freezed
abstract class MemoryState with _$MemoryState {
  const factory MemoryState({
    required AsyncValue<List<MemoryItem>> items,
    @Default(false) bool saving,
    Object? actionError,
  }) = _MemoryState;

  factory MemoryState.initial() => const MemoryState(items: AsyncLoading());

  AsyncValue<List<MemoryItem>> get items;
  bool get saving;
  Object? get actionError;
}
