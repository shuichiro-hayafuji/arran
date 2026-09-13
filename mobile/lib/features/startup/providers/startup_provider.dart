import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../screens/startup/startup_state.dart';
import '../screens/startup/startup_view_model.dart';

@riverpod
StartupViewModel startupViewModelFactory(Ref ref) => StartupViewModel(ref);

final startupViewModelProvider =
    StateNotifierProvider<StartupViewModel, StartupState>(
      startupViewModelFactory,
    );
