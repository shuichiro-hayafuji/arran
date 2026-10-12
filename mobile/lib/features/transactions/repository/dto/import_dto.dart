import 'dart:convert';

import 'package:dio/dio.dart';

import '../../../../shared/utils/json_parsing.dart';
import '../../domain/csv_mapping.dart';
import '../../domain/import_commit_result.dart';
import '../../domain/import_preview.dart';
import 'transaction_item_dto.dart';

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
    this.identityConflicts = const [],
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

  final List<ImportIdentityConflict> identityConflicts;
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
    identityConflicts: identityConflicts,
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
    required this.importedCount,
    required this.duplicateCount,
  });

  factory ImportCommitResultDto.fromJson(Map<String, dynamic> json) =>
      ImportCommitResultDto(
        importedCount: parseInt(json['imported_count']),
        duplicateCount: parseInt(json['duplicate_count']),
      );

  final int importedCount;
  final int duplicateCount;

  ImportCommitResult toDomain() => ImportCommitResult(
    importedCount: importedCount,
    duplicateCount: duplicateCount,
  );
}

class ImportPreviewRequestDto {
  const ImportPreviewRequestDto({
    required this.filePath,
    required this.fileName,
    required this.source,
    required this.accountName,
    required this.mapping,
    this.identityDecisions = const [],
  });

  final String filePath;
  final String fileName;
  final String source;
  final String accountName;
  final CsvMapping? mapping;
  final List<ImportIdentityDecision> identityDecisions;

  FormData toFormData() => FormData.fromMap({
    'file': MultipartFile.fromFileSync(filePath, filename: fileName),
    'source': source,
    'source_account_name': accountName,
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
