import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';
import '../domain/import_management.dart';
import 'dto/import_management_dto.dart';

abstract interface class ImportManagementRepository {
  Future<List<ImportBatch>> history(String before);
  Future<ImportBatch> batch(String id);
  Future<TransactionRecord> transaction(int id);
  Future<List<TransactionAudit>> audit(String target);
  Future<MutationResult> mutate(MutationRequest request);
  Future<MutationResult> result(String id);
}

class ImportManagementRepositoryImpl implements ImportManagementRepository {
  const ImportManagementRepositoryImpl(this.client);
  final ApiClient client;
  @override
  Future<List<ImportBatch>> history(String before) async => List.unmodifiable(
    parseMaps(
      (await client.get('/imports', queryParameters: {'before': before})).data,
    ).map(importBatchFromJson),
  );
  @override
  Future<ImportBatch> batch(String id) async =>
      importBatchFromJson(parseMap((await client.get('/imports/$id')).data));
  @override
  Future<TransactionRecord> transaction(int id) async =>
      transactionRecordFromJson(
        parseMap((await client.get('/transactions/$id')).data),
      );
  @override
  Future<List<TransactionAudit>> audit(String target) async =>
      List.unmodifiable(
        parseMaps(
          (await client.get('/changes/$target')).data,
        ).map(transactionAuditFromJson),
      );
  @override
  Future<MutationResult> mutate(MutationRequest request) async {
    final data = mutationRequestToJson(request);
    final response = request.action == 'correct'
        ? await client.patch('/transactions/${request.target}', data: data)
        : await client.post(
            '/imports/${request.target}/${request.action}',
            data: data,
          );
    return mutationResultFromJson(parseMap(response.data));
  }

  @override
  Future<MutationResult> result(String id) async => mutationResultFromJson(
    parseMap((await client.get('/mutations/$id')).data),
  );
}

final importManagementRepositoryProvider = Provider<ImportManagementRepository>(
  (ref) =>
      ImportManagementRepositoryImpl(ref.watch(authorizedApiClientProvider)),
);
