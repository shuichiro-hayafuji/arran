import 'dart:convert';

import 'package:dio/dio.dart';

import '../../../../shared/utils/json_parsing.dart';
import '../../domain/csv_mapping.dart';
import '../../domain/import_commit_result.dart';
import '../../domain/import_preview.dart';
import '../../domain/expense_count.dart';
import 'transaction_item_dto.dart';

class ExpenseCountDto {
  const ExpenseCountDto(
    this.confirmed,
    this.estimated,
    this.undated,
    this.unresolved,
    this.pending,
  );
  factory ExpenseCountDto.fromJson(Map<String, dynamic> json) =>
      ExpenseCountDto(
        parseInt(json['confirmed']),
        parseInt(json['estimated']),
        parseInt(json['undated']),
        parseInt(json['unresolved']),
        parseInt(json['pending_recalculation']),
      );
  final int confirmed, estimated, undated, unresolved, pending;
  ExpenseCount toDomain() => ExpenseCount(
    confirmed: confirmed,
    estimated: estimated,
    undated: undated,
    unresolved: unresolved,
    pendingRecalculation: pending,
  );
}

class CsvMappingDto {
  const CsvMappingDto({
    this.transactionIdColumn = "",
    this.dateColumn = '',
    this.descriptionColumn = '',
    this.amountColumn = '',
    this.debitColumn = '',
    this.creditColumn = '',
  });

  factory CsvMappingDto.fromDomain(CsvMapping mapping) => CsvMappingDto(
    transactionIdColumn: mapping.transactionIdColumn,
    dateColumn: mapping.dateColumn,
    descriptionColumn: mapping.descriptionColumn,
    amountColumn: mapping.amountColumn,
    debitColumn: mapping.debitColumn,
    creditColumn: mapping.creditColumn,
  );

  factory CsvMappingDto.fromJson(Map<String, dynamic> json) => CsvMappingDto(
    transactionIdColumn: json['transaction_id_column'] as String? ?? '',
    dateColumn: json['date_column'] as String? ?? '',
    descriptionColumn: json['description_column'] as String? ?? '',
    amountColumn: json['amount_column'] as String? ?? '',
    debitColumn: json['debit_column'] as String? ?? '',
    creditColumn: json['credit_column'] as String? ?? '',
  );

  final String transactionIdColumn;
  final String dateColumn;
  final String descriptionColumn;
  final String amountColumn;
  final String debitColumn;
  final String creditColumn;

  Map<String, dynamic> toJson() => {
    'transaction_id_column': transactionIdColumn,
    'date_column': dateColumn,
    'description_column': descriptionColumn,
    'amount_column': amountColumn,
    'debit_column': debitColumn,
    'credit_column': creditColumn,
  };
}

class ImportPreviewDto {
  const ImportPreviewDto({
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

  factory ImportPreviewDto.fromJson(Map<String, dynamic> json) =>
      ImportPreviewDto(
        reactivationIds: (json["reactivation_ids"] as List? ?? []).cast<int>(),
        reconciliationConflicts: parseMaps(
          json["reconciliation_conflicts"],
        ).map(purchaseCsvConflictFromJson).toList(),
        identityConflicts: parseMaps(json["identity_conflicts"])
            .map(
              (value) => ImportIdentityConflict(
                candidateSnapshots: parseMap(value["candidate_snapshots"]).map(
                  (key, value) => MapEntry(int.parse(key), value as String),
                ),
                row: parseInt(value["row"]),
                transaction: TransactionItemDto.fromJson(
                  parseMap(value["transaction"]),
                ).toDomain(),
                candidates: parseMaps(value["candidates"])
                    .map((t) => TransactionItemDto.fromJson(t).toDomain())
                    .toList(),
              ),
            )
            .toList(),
        identityNotice: json["identity_notice"] as String? ?? "",
        previewId: json['preview_id'] as String? ?? '',
        detectedEncoding: json['detected_encoding'] as String? ?? '',
        headers: parseStrings(json['headers']),
        mapping: CsvMappingDto.fromJson(parseMap(json['mapping'])),
        needsMapping: json['needs_mapping'] as bool? ?? false,
        readCount: parseInt(json['read_count']),
        periodStart: json['period_start'] as String? ?? '',
        periodEnd: json['period_end'] as String? ?? '',
        expenseTotal: parseInt(json['expense_total']),
        rows: parseMaps(json['rows']).map(TransactionItemDto.fromJson).toList(),
        warnings: parseStrings(json['warnings']),
      );

  final List<int> reactivationIds;
  final List<ImportIdentityConflict> identityConflicts;
  final List<PurchaseCsvConflict> reconciliationConflicts;
  final String identityNotice;
  final String previewId;
  final String detectedEncoding;
  final List<String> headers;
  final CsvMappingDto mapping;
  final bool needsMapping;
  final int readCount;
  final String periodStart;
  final String periodEnd;
  final int expenseTotal;
  final List<TransactionItemDto> rows;
  final List<String> warnings;

  ImportPreview toDomain() => ImportPreview(
    reactivationIds: List.unmodifiable(reactivationIds),
    identityConflicts: identityConflicts,
    reconciliationConflicts: reconciliationConflicts,
    identityNotice: identityNotice,
    previewId: previewId,
    detectedEncoding: detectedEncoding,
    headers: headers,
    mapping: CsvMapping(
      transactionIdColumn: mapping.transactionIdColumn,
      dateColumn: mapping.dateColumn,
      descriptionColumn: mapping.descriptionColumn,
      amountColumn: mapping.amountColumn,
      debitColumn: mapping.debitColumn,
      creditColumn: mapping.creditColumn,
    ),
    needsMapping: needsMapping,
    readCount: readCount,
    periodStart: periodStart,
    periodEnd: periodEnd,
    expenseTotal: expenseTotal,
    rows: rows.map((dto) => dto.toDomain()).toList(),
    warnings: warnings,
  );
}

class ImportCommitResultDto {
  const ImportCommitResultDto({
    this.importId = "",
    this.reactivatedCount = 0,
    this.financialPending = 0,
    required this.importedCount,
    required this.duplicateCount,
  });

  factory ImportCommitResultDto.fromJson(Map<String, dynamic> json) =>
      ImportCommitResultDto(
        importId: json["import_id"] as String? ?? "",
        reactivatedCount: json["reactivated_count"] as int? ?? 0,
        financialPending: json["financial_pending"] as int? ?? 0,
        importedCount: parseInt(json['imported_count']),
        duplicateCount: parseInt(json['duplicate_count']),
      );

  final String importId;
  final int reactivatedCount, financialPending;
  final int importedCount;
  final int duplicateCount;

  ImportCommitResult toDomain() => ImportCommitResult(
    importId: importId,
    reactivatedCount: reactivatedCount,
    financialPending: financialPending,
    importedCount: importedCount,
    duplicateCount: duplicateCount,
  );
}

class ImportPreviewRequestDto {
  const ImportPreviewRequestDto({
    this.confirmReactivation = false,
    required this.filePath,
    required this.fileName,
    required this.source,
    required this.accountName,
    required this.mapping,
    this.identityDecisions = const [],
    this.reconciliationDecisions = const [],
  });

  final bool confirmReactivation;
  final String filePath;
  final String fileName;
  final String source;
  final String accountName;
  final CsvMapping? mapping;
  final List<ImportIdentityDecision> identityDecisions;
  final List<PurchaseCsvDecision> reconciliationDecisions;

  FormData toFormData() => FormData.fromMap({
    'file': MultipartFile.fromFileSync(filePath, filename: fileName),
    'source': source,
    'confirm_reactivation': confirmReactivation.toString(),
    'source_account_name': accountName,
    'reconciliation_decisions': jsonEncode(
      reconciliationDecisions.map(purchaseCsvDecisionToJson).toList(),
    ),
    'identity_decisions': jsonEncode(
      identityDecisions
          .map(
            (d) => {
              'row': d.row,
              'action': d.action,
              'transaction_id': d.transactionId,
              'snapshot': d.snapshot,
            },
          )
          .toList(),
    ),
    if (mapping != null)
      'mapping': jsonEncode(CsvMappingDto.fromDomain(mapping!).toJson()),
  });
}

PurchaseCsvConflict purchaseCsvConflictFromJson(Map<String, dynamic> json) =>
    PurchaseCsvConflict(
      row: parseInt(json['row']),
      transaction: TransactionItemDto.fromJson(
        parseMap(json['transaction']),
      ).toDomain(),
      stateVersion: parseInt(json['state_version']),
      candidates: parseMaps(json['candidates'])
          .map(
            (c) => PurchaseCsvCandidate(
              id: c['id'] as String,
              description: c['description'] as String? ?? '',
              amount:
                  parseNullableInt(c['actual_amount']) ??
                  parseInt(c['planned_amount']),
              recordedAt: c['recorded_at'] as String? ?? '',
              purchaseDate: c['purchase_date'] as String? ?? '',
              status: c['status'] as String? ?? '',
              snapshot: c['snapshot'] as String,
            ),
          )
          .toList(),
    );
Map<String, dynamic> purchaseCsvDecisionToJson(PurchaseCsvDecision d) => {
  'row': d.row,
  'action': d.action,
  'purchase_id': d.purchaseId,
  'snapshot': d.snapshot,
  'state_version': d.stateVersion,
};
