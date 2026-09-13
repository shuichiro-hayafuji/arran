import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../screens/memories/memory_state.dart';
import '../screens/memories/memory_view_model.dart';

@riverpod
MemoryViewModel memoryViewModelFactory(Ref ref) => MemoryViewModel(ref);

final memoriesViewModelProvider =
    StateNotifierProvider<MemoryViewModel, MemoryState>(memoryViewModelFactory);
