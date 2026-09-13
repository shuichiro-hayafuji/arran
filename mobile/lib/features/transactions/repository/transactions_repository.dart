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
    required String filePath,
    required String fileName,
    required String source,
    required String accountName,
    CsvMapping? mapping,
  });
  Future<ImportCommitResult> commitImport(String previewId);
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
    required String filePath,
    required String fileName,
    required String source,
    required String accountName,
    CsvMapping? mapping,
  }) async {
    final request = ImportPreviewRequestDto(
      filePath: filePath,
      fileName: fileName,
      source: source,
      accountName: accountName,
      mapping: mapping,
    );
    final response = await _client.post(
      '/transactions/import/preview',
      data: request.toFormData(),
      options: Options(contentType: 'multipart/form-data'),
    );
    return ImportPreviewDto.fromJson(parseMap(response.data)).toDomain();
  }

  @override
  Future<ImportCommitResult> commitImport(String previewId) async {
    final response = await _client.post(
      '/transactions/import/commit',
      data: {'preview_id': previewId},
    );
    return ImportCommitResultDto.fromJson(parseMap(response.data)).toDomain();
  }
}
