import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/dashboard/domain/dashboard.dart';

part 'dashboard_state.freezed.dart';

@freezed
abstract class DashboardState with _$DashboardState {
  const factory DashboardState({required AsyncValue<Dashboard> data}) =
      _DashboardState;

  factory DashboardState.initial() =>
      const DashboardState(data: AsyncLoading());

  AsyncValue<Dashboard> get data;
}
