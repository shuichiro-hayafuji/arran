import '../../domain/import_management.dart';

class ManagementState {
  const ManagementState({
    this.batch,
    this.record,
    this.audit = const [],
    this.loading = false,
    this.pending,
    this.result,
    this.error,
  });
  final ImportBatch? batch;
  final TransactionRecord? record;
  final List<TransactionAudit> audit;
  final bool loading;
  final MutationRequest? pending;
  final MutationResult? result;
  final String? error;
}
