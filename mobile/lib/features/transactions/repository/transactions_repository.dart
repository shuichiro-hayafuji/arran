import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';
import '../domain/csv_mapping.dart';
import '../domain/import_commit_result.dart';
import '../domain/import_preview.dart';
import '../domain/transaction_item.dart';
import 'dto/import_dto.dart';
import 'dto/transaction_item_dto.dart';

@riverpod
TransactionsRepository transactionsRepository(Ref ref) =>
    TransactionsRepositoryImpl(ref.watch(authorizedApiClientProvider));

final transactionsRepositoryProvider = Provider<TransactionsRepository>(
  transactionsRepository,
);

abstract interface class TransactionsRepository {
  Future<List<TransactionItem>> getAll();
  Future<TransactionItem> updateCategory(
    int id,
    String category, {
    required bool rememberMerchant,
  });
  Future<ImportPreview> previewImport({
    int? sessionGeneration,
    bool confirmReactivation = false,
    required String filePath,
    required String fileName,
    required String source,
    required String accountName,
    CsvMapping? mapping,
    List<ImportIdentityDecision> identityDecisions = const [],
    List<PurchaseCsvDecision> reconciliationDecisions = const [],
  });
  Future<ImportCommitResult> commitImport(
    String previewId, {
    required int sessionGeneration,
  });
}

class TransactionsRepositoryImpl implements TransactionsRepository {
  const TransactionsRepositoryImpl(this._client);

  final ApiClient _client;

  @override
  Future<List<TransactionItem>> getAll() async {
    final response = await _client.get(
      '/transactions',
      queryParameters: {'limit': 500},
    );
    return parseMaps(
      parseMap(response.data)['items'],
    ).map(TransactionItemDto.fromJson).map((dto) => dto.toDomain()).toList();
  }

  @override
  Future<TransactionItem> updateCategory(
    int id,
    String category, {
    required bool rememberMerchant,
  }) async {
    final response = await _client.patch(
      '/transactions/$id',
      data: {'category': category, 'remember_merchant': rememberMerchant},
    );
    return TransactionItemDto.fromJson(parseMap(response.data)).toDomain();
  }

  @override
  Future<ImportPreview> previewImport({
    int? sessionGeneration,
    bool confirmReactivation = false,
    required String filePath,
    required String fileName,
    required String source,
    required String accountName,
    CsvMapping? mapping,
    List<ImportIdentityDecision> identityDecisions = const [],
    List<PurchaseCsvDecision> reconciliationDecisions = const [],
  }) async {
    final request = ImportPreviewRequestDto(
      confirmReactivation: confirmReactivation,
      filePath: filePath,
      fileName: fileName,
      source: source,
      accountName: accountName,
      identityDecisions: identityDecisions,
      reconciliationDecisions: reconciliationDecisions,
      mapping: mapping,
    );
    final response = await _client.post(
      '/transactions/import/preview',
      data: request.toFormData(),
      options: Options(
        contentType: 'multipart/form-data',
        extra: {
          if (sessionGeneration != null)
            expectedSessionGenerationKey: sessionGeneration,
        },
      ),
    );
    return ImportPreviewDto.fromJson(parseMap(response.data)).toDomain();
  }

  @override
  Future<ImportCommitResult> commitImport(
    String previewId, {
    required int sessionGeneration,
  }) async {
    final response = await _client.post(
      '/transactions/import/commit',
      data: {'preview_id': previewId},
      options: Options(
        extra: {expectedSessionGenerationKey: sessionGeneration},
      ),
    );
    final data = response.data;
    if (response.statusCode != 200 ||
        data is! Map ||
        data['import_id'] is! String ||
        data['imported_count'] is! int ||
        data['duplicate_count'] is! int ||
        data['reactivated_count'] is! int ||
        data['financial_pending'] is! int ||
        (data['imported_count'] as int) < 0 ||
        (data['duplicate_count'] as int) < 0 ||
        (data['reactivated_count'] as int) < 0 ||
        (data['financial_pending'] as int) < 0 ||
        ((data['import_id'] as String).trim().isEmpty &&
            ((data['imported_count'] as int) > 0 ||
                (data['duplicate_count'] as int) > 0 ||
                (data['reactivated_count'] as int) > 0 ||
                (data['financial_pending'] as int) > 0))) {
      throw const ApiException('保存結果を確認できませんでした。');
    }
    return ImportCommitResultDto.fromJson(parseMap(data)).toDomain();
  }
}
