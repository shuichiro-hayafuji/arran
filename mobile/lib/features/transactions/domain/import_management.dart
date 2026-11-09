import 'transaction_item.dart';

class ImportBatch {
  const ImportBatch({
    required this.id,
    required this.fileName,
    required this.source,
    required this.account,
    required this.status,
    required this.version,
    required this.stateVersion,
    required this.snapshot,
    required this.recordedAt,
    required this.legacy,
    required this.affected,
    required this.protected,
    required this.rows,
  });
  final String id, fileName, source, account, status, snapshot, recordedAt;
  final int version, stateVersion, affected, protected;
  final bool legacy;
  final List<ImportEvidence> rows;
}

class ImportEvidence {
  const ImportEvidence(
    this.row,
    this.transactionId,
    this.original,
    this.current,
    this.active,
    this.legacyRetained,
  );
  final int row, transactionId;
  final TransactionItem original, current;
  final bool active, legacyRetained;
}

class TransactionRecord {
  const TransactionRecord(
    this.original,
    this.current,
    this.active,
    this.stateVersion,
    this.revision,
  );
  final TransactionItem original, current;
  final bool active;
  final int stateVersion, revision;
}

class MutationRequest {
  const MutationRequest({
    required this.id,
    required this.target,
    required this.action,
    required this.stateVersion,
    required this.version,
    this.snapshot = '',
    this.amount,
    this.date,
    this.category,
    this.remember = false,
  });
  final String id, target, action, snapshot;
  final int stateVersion, version;
  final int? amount;
  final String? date, category;
  final bool remember;
}

class MutationResult {
  const MutationResult(this.id, this.affected, this.protected, this.pending);
  final String id;
  final int affected, protected, pending;
}

class TransactionAudit {
  const TransactionAudit(this.kind, this.recordedAt, this.before, this.after);
  final String kind, recordedAt;
  final List<TransactionItem> before, after;
}
