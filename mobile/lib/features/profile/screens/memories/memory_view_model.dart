import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:spendable_today/features/profile/domain/memory_item.dart';
import 'package:spendable_today/features/profile/providers/memory_repository_provider.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import 'memory_intent.dart';
import 'memory_state.dart';

class MemoryViewModel extends MviViewModel<MemoryState, MemoryIntent> {
  MemoryViewModel(this.ref) : super(MemoryState.initial()) {
    dispatch(const LoadMemories());
  }

  final Ref ref;

  @override
  Future<void> dispatch(MemoryIntent intent) {
    switch (intent) {
      case LoadMemories():
      case RefreshMemories():
        return _load();
      case SaveMemory(item: final item):
        return _save(item);
      case DeleteMemory(id: final id):
        return _delete(id);
    }
  }

  Future<void> _load() async {
    state = state.copyWith(items: const AsyncLoading(), actionError: null);
    try {
      final loaded = await ref.read(memoryRepositoryProvider).getAll();
      if (!mounted) return;
      state = state.copyWith(items: AsyncData(loaded), actionError: null);
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = state.copyWith(items: AsyncError(error, stackTrace));
    }
  }

  Future<void> _save(MemoryItem item) async {
    state = state.copyWith(saving: true, actionError: null);
    try {
      await ref.read(memoryRepositoryProvider).save(item);
      if (!mounted) return;
      await _load();
      if (!mounted) return;
    } catch (error) {
      if (!mounted) return;
      state = state.copyWith(actionError: error);
    } finally {
      if (mounted) state = state.copyWith(saving: false);
    }
  }

  Future<void> _delete(int id) async {
    state = state.copyWith(saving: true, actionError: null);
    try {
      await ref.read(memoryRepositoryProvider).delete(id);
      if (!mounted) return;
      await _load();
      if (!mounted) return;
    } catch (error) {
      if (!mounted) return;
      state = state.copyWith(actionError: error);
    } finally {
      if (mounted) state = state.copyWith(saving: false);
    }
  }
}
