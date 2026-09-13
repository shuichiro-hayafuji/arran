import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../repository/dashboard_repository.dart';

@riverpod
DashboardRepository dashboardRepository(Ref ref) =>
    DashboardRepositoryImpl(ref.watch(authorizedApiClientProvider));

final dashboardRepositoryProvider = Provider<DashboardRepository>(
  dashboardRepository,
);
