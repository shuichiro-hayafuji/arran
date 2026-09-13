import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';
import '../domain/dashboard.dart';
import 'dto/dashboard_dto.dart';

@riverpod
DashboardRepository dashboardRepository(Ref ref) =>
    DashboardRepositoryImpl(ref.watch(authorizedApiClientProvider));

final dashboardRepositoryProvider = Provider<DashboardRepository>(
  dashboardRepository,
);

abstract interface class DashboardRepository {
  Future<Dashboard> getMonthly();
}

class DashboardRepositoryImpl implements DashboardRepository {
  const DashboardRepositoryImpl(this._client);

  final ApiClient _client;

  @override
  Future<Dashboard> getMonthly() async {
    final response = await _client.get('/dashboard/monthly');
    return DashboardDto.fromJson(parseMap(response.data)).toDomain();
  }
}
