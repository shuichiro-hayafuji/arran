import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';
import '../domain/dashboard.dart';
import 'dto/dashboard_dto.dart';

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
