import 'dart:convert';
import '../../domain/import_management.dart';
import 'transaction_item_dto.dart';
import '../../../../shared/utils/json_parsing.dart';

TransactionRecord transactionRecordFromJson(Map<String, dynamic> json) =>
    TransactionRecord(
      TransactionItemDto.fromJson(parseMap(json['original'])).toDomain(),
      TransactionItemDto.fromJson(parseMap(json['current'])).toDomain(),
      json['active'] as bool,
      json['state_version'] as int,
      parseMap(json['current'])['revision'] as int,
    );
ImportBatch importBatchFromJson(Map<String, dynamic> json) => ImportBatch(
  id: json['id'] as String,
  fileName: parseMap(json['metadata'])['file_name'] as String? ?? '',
  source: json['source'] as String,
  account: json['account'] as String,
  status: json['status'] as String,
  version: json['version'] as int,
  stateVersion: json['state_version'] as int,
  snapshot: json['snapshot'] as String,
  recordedAt: json['recorded_at'] as String,
  legacy: json['legacy'] as bool,
  affected: json['cancel_affected_count'] as int,
  protected: json['cancel_protected_count'] as int,
  rows: List.unmodifiable(
    parseMaps(json['rows']).map(
      (e) => ImportEvidence(
        e['row'] as int,
        e['transaction_id'] as int,
        TransactionItemDto.fromJson(parseMap(e['original'])).toDomain(),
        TransactionItemDto.fromJson(parseMap(e['current'])).toDomain(),
        e['active'] as bool,
        e['legacy_retained'] as bool,
      ),
    ),
  ),
);
MutationResult mutationResultFromJson(Map<String, dynamic> json) =>
    MutationResult(
      json['mutation_id'] as String,
      json['affected_count'] as int,
      json['protected_count'] as int,
      json['pending_recalculation'] as int,
    );
Map<String, dynamic> mutationRequestToJson(MutationRequest request) => {
  'mutation_id': request.id,
  'expected_version': request.version,
  'state_version': request.stateVersion,
  'snapshot': request.snapshot,
  if (request.amount != null) 'amount': request.amount,
  if (request.date != null) 'transaction_date': request.date,
  if (request.category != null) 'category': request.category,
  'remember_merchant': request.remember,
};
TransactionAudit transactionAuditFromJson(Map<String, dynamic> json) =>
    TransactionAudit(
      json['kind'] as String,
      json['recorded_at'] as String,
      List.unmodifiable(
        parseMaps(
          jsonDecode(json['before_state'] as String),
        ).map((v) => TransactionItemDto.fromJson(v).toDomain()),
      ),
      List.unmodifiable(
        parseMaps(
          jsonDecode(json['after_state'] as String),
        ).map((v) => TransactionItemDto.fromJson(v).toDomain()),
      ),
    );
