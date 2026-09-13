import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/shared/presentation/mvi.dart';

part 'dashboard_intent.freezed.dart';

@freezed
sealed class DashboardIntent with _$DashboardIntent implements MviIntent {
  const DashboardIntent._();

  const factory DashboardIntent.load() = LoadDashboard;
  const factory DashboardIntent.refresh() = RefreshDashboard;
}
