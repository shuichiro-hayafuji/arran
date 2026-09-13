import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../screens/dashboard/dashboard_view_model.dart';
import '../screens/dashboard/dashboard_state.dart';

@riverpod
DashboardViewModel dashboardViewModelFactory(Ref ref) =>
    DashboardViewModel(ref);

final dashboardViewModelProvider =
    StateNotifierProvider<DashboardViewModel, DashboardState>(
      dashboardViewModelFactory,
    );
