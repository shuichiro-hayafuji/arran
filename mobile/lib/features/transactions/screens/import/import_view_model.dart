import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/dashboard/providers/dashboard_provider.dart';
import 'package:spendable_today/features/dashboard/screens/dashboard/dashboard_intent.dart';
import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/features/transactions/domain/import_commit_result.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/providers/transactions_repository_provider.dart';
import 'package:spendable_today/features/transactions/screens/list/transactions_intent.dart';
import 'package:spendable_today/features/transactions/providers/transactions_provider.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import 'import_intent.dart';

part 'import_view_model.freezed.dart';

@freezed
abstract class ImportState with _$ImportState {
  const factory ImportState({
    String? filePath,
    String? fileName,
    ImportPreview? preview,
    ImportCommitResult? commitResult,
    @Default(false) bool loading,
    String? error,
  }) = _ImportState;

  String? get filePath;
  String? get fileName;
  ImportPreview? get preview;
  ImportCommitResult? get commitResult;
  bool get loading;
  String? get error;
}

/// CSV の選択・列の再割当・プレビュー確認・確定を管理する。
/// 確定にはサーバー発行の previewId を使い、成功後に明細と月次集計を再取得する。
class ImportViewModel extends MviViewModel<ImportState, ImportIntent> {
  ImportViewModel(this.ref) : super(const ImportState());

  final Ref ref;

  @override
  Future<void> dispatch(ImportIntent intent) {
    switch (intent) {
      case SelectImportFile(filePath: final path, fileName: final name):
        return selectAndPreview(filePath: path, fileName: name);
      case ApplyImportMapping(mapping: final mapping):
        return remap(mapping);
      case CommitImport():
        return commit();
      case ResetImport():
        reset();
        return Future.value();
    }
  }

  Future<void> selectAndPreview({
    required String filePath,
    required String fileName,
    CsvMapping? mapping,
  }) async {
    state = state.copyWith(
      filePath: filePath,
      fileName: fileName,
      loading: true,
      error: null,
    );
    await _preview(mapping);
    if (!mounted) return;
  }

  Future<void> remap(CsvMapping mapping) async {
    state = state.copyWith(loading: true, error: null);
    await _preview(mapping);
    if (!mounted) return;
  }

  Future<void> _preview(CsvMapping? mapping) async {
    try {
      final preview = await ref
          .read(transactionsRepositoryProvider)
          .previewImport(
            filePath: state.filePath!,
            fileName: state.fileName!,
            source: 'manual_csv',
            accountName: '個人',
            mapping: mapping,
          );
      if (!mounted) return;
      state = state.copyWith(preview: preview, loading: false, error: null);
    } catch (error) {
      if (!mounted) return;
      state = state.copyWith(loading: false, error: error.toString());
    }
  }

  Future<void> commit() async {
    final preview = state.preview;
    if (preview == null || preview.previewId.isEmpty) {
      state = state.copyWith(error: '有効なプレビューがありません。');
      return;
    }
    state = state.copyWith(loading: true, error: null);
    try {
      final result = await ref
          .read(transactionsRepositoryProvider)
          .commitImport(preview.previewId);
      if (!mounted) return;
      state = state.copyWith(commitResult: result, loading: false, error: null);
      ref
          .read(transactionsViewModelProvider.notifier)
          .dispatch(const RefreshTransactions());
      ref
          .read(dashboardViewModelProvider.notifier)
          .dispatch(const RefreshDashboard());
    } catch (error) {
      if (!mounted) return;
      state = state.copyWith(loading: false, error: error.toString());
    }
  }

  void reset() => state = const ImportState();
}
