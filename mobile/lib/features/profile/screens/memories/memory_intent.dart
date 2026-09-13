import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/profile/domain/memory_item.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';

part 'memory_intent.freezed.dart';

@freezed
sealed class MemoryIntent with _$MemoryIntent implements MviIntent {
  const MemoryIntent._();

  const factory MemoryIntent.load() = LoadMemories;
  const factory MemoryIntent.refresh() = RefreshMemories;
  const factory MemoryIntent.save(MemoryItem item) = SaveMemory;
  const factory MemoryIntent.delete(int id) = DeleteMemory;
}
