import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';
import '../domain/import_preview.dart';
import '../domain/expense_count.dart';
import 'dto/import_dto.dart';

abstract interface class ReconciliationRepository {
  Future<List<PurchaseCsvConflict>> candidates(String purchaseId);
  Future<void> confirm(int transactionId, PurchaseCsvDecision decision);
  Future<ExpenseCount> count();
}

class ReconciliationRepositoryImpl implements ReconciliationRepository {
  const ReconciliationRepositoryImpl(this.client);
  final ApiClient client;
  @override
  Future<List<PurchaseCsvConflict>> candidates(String purchaseId) async {
    final response = await client.get('/purchases/$purchaseId/csv-candidates');
    return parseMaps(response.data).map(purchaseCsvConflictFromJson).toList();
  }

  @override
  Future<void> confirm(int transactionId, PurchaseCsvDecision decision) async {
    final path = transactionId == 0
        ? '/purchases/${decision.purchaseId}/csv-distinct'
        : '/transactions/$transactionId/reconciliation';
    await client.post(path, data: purchaseCsvDecisionToJson(decision));
  }

  @override
  Future<ExpenseCount> count() async {
    final response = await client.get('/expenses/count');
    return ExpenseCountDto.fromJson(parseMap(response.data)).toDomain();
  }
}

final reconciliationRepositoryProvider = Provider<ReconciliationRepository>(
  (ref) => ReconciliationRepositoryImpl(ref.watch(authorizedApiClientProvider)),
);
final expenseCountProvider = FutureProvider.autoDispose<ExpenseCount>(
  (ref) => ref.watch(reconciliationRepositoryProvider).count(),
);
