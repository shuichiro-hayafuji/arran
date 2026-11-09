import 'csv_mapping.dart';
import 'transaction_item.dart';

class ImportPreview {
  const ImportPreview({
    this.reactivationIds = const [],
    this.identityConflicts = const [],
    this.reconciliationConflicts = const [],
    this.identityNotice = "",
    required this.previewId,
    required this.detectedEncoding,
    required this.headers,
    required this.mapping,
    required this.needsMapping,
    required this.readCount,
    required this.periodStart,
    required this.periodEnd,
    required this.expenseTotal,
    required this.rows,
    required this.warnings,
  });

  final List<int> reactivationIds;
  final List<ImportIdentityConflict> identityConflicts;
  final List<PurchaseCsvConflict> reconciliationConflicts;
  final String identityNotice;
  final String previewId;
  final String detectedEncoding;
  final List<String> headers;
  final CsvMapping mapping;
  final bool needsMapping;
  final int readCount;
  final String periodStart;
  final String periodEnd;
  final int expenseTotal;
  final List<TransactionItem> rows;
  final List<String> warnings;
}

class ImportIdentityDecision {
  const ImportIdentityDecision({
    required this.row,
    required this.action,
    this.transactionId = 0,
    this.snapshot = "",
  });
  final int row;
  final String action;
  final int transactionId;
  final String snapshot;
}

class ImportIdentityConflict {
  const ImportIdentityConflict({
    required this.row,
    required this.transaction,
    required this.candidates,
    this.candidateSnapshots = const {},
  });
  final int row;
  final TransactionItem transaction;
  final List<TransactionItem> candidates;
  final Map<int, String> candidateSnapshots;
}

class PurchaseCsvDecision {
  const PurchaseCsvDecision({
    required this.row,
    required this.action,
    required this.stateVersion,
    this.purchaseId = '',
    this.snapshot = '',
  });
  final int row;
  final String action;
  final String purchaseId;
  final String snapshot;
  final int stateVersion;
}

class PurchaseCsvCandidate {
  const PurchaseCsvCandidate({
    required this.id,
    required this.description,
    required this.amount,
    required this.recordedAt,
    required this.snapshot,
    this.status = '',
    this.purchaseDate = '',
  });
  final String id;
  final String description;
  final int amount;
  final String recordedAt;
  final String snapshot;
  final String status;
  final String purchaseDate;
}

class PurchaseCsvConflict {
  const PurchaseCsvConflict({
    required this.row,
    required this.transaction,
    required this.candidates,
    required this.stateVersion,
  });
  final int row;
  final TransactionItem transaction;
  final List<PurchaseCsvCandidate> candidates;
  final int stateVersion;
}
