import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:spendable_today/features/dashboard/providers/dashboard_repository_provider.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import 'dashboard_intent.dart';
import 'dashboard_state.dart';

class DashboardViewModel extends MviViewModel<DashboardState, DashboardIntent> {
  DashboardViewModel(this.ref) : super(DashboardState.initial()) {
    dispatch(const LoadDashboard());
  }

  final Ref ref;

  @override
  Future<void> dispatch(DashboardIntent intent) {
    switch (intent) {
      case LoadDashboard():
      case RefreshDashboard():
        return _load();
    }
  }

  Future<void> _load() async {
    state = state.copyWith(data: const AsyncLoading());
    try {
      final loaded = await ref.read(dashboardRepositoryProvider).getMonthly();
      if (!mounted) return;
      state = state.copyWith(data: AsyncData(loaded));
    } catch (error, stackTrace) {
      if (!mounted) return;
      state = state.copyWith(data: AsyncError(error, stackTrace));
    }
  }
}
