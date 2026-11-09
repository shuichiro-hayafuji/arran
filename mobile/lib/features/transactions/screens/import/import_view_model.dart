import '../../repository/reconciliation_repository.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/dashboard/screens/dashboard/dashboard_view_model.dart';
import 'package:spendable_today/features/dashboard/screens/dashboard/dashboard_intent.dart';
import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/features/transactions/domain/import_commit_result.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/repository/transactions_repository.dart';
import 'package:spendable_today/features/transactions/screens/list/transactions_intent.dart';
import 'package:spendable_today/features/transactions/screens/list/transactions_view_model.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';
import 'import_intent.dart';

part 'import_view_model.freezed.dart';

@riverpod
ImportViewModel importViewModelFactory(Ref ref) => ImportViewModel(ref);

final importControllerProvider =
    StateNotifierProvider<ImportViewModel, ImportState>(importViewModelFactory);

@freezed
abstract class ImportState with _$ImportState {
  const factory ImportState({
    @Default(false) bool confirmReactivation,
    @Default("manual_csv") String source,
    @Default("個人") String accountName,
    @Default(<ImportIdentityDecision>[])
    List<ImportIdentityDecision> identityDecisions,
    @Default(<PurchaseCsvDecision>[])
    List<PurchaseCsvDecision> reconciliationDecisions,
    String? filePath,
    String? fileName,
    ImportPreview? preview,
    ImportCommitResult? commitResult,
    @Default(false) bool loading,
    String? error,
  }) = _ImportState;

  bool get confirmReactivation;
  String get source;
  String get accountName;
  List<ImportIdentityDecision> get identityDecisions;
  List<PurchaseCsvDecision> get reconciliationDecisions;
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
      case SetImportSource(source: final source, accountName: final account):
        return setSource(source, account);
      case ResolveImportIdentities(decisions: final decisions):
        return resolve(decisions);
      case ResolveImportPurchases(decisions: final decisions):
        return resolvePurchases(decisions);
      case ConfirmImportReactivation():
        if (state.loading || state.preview == null) return Future.value();
        state = state.copyWith(confirmReactivation: true);
        return _preview(state.preview!.mapping);
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
    if (state.loading) return;
    state = state.copyWith(
      confirmReactivation: false,
      identityDecisions: [],
      reconciliationDecisions: [],
      preview: null,
      commitResult: null,
      filePath: filePath,
      fileName: fileName,
      loading: true,
      error: null,
    );
    await _preview(mapping);
    if (!mounted) return;
  }

  Future<void> remap(CsvMapping mapping) async {
    if (state.loading) return;
    state = state.copyWith(
      confirmReactivation: false,
      identityDecisions: [],
      reconciliationDecisions: [],
      preview: null,
      commitResult: null,
      loading: true,
      error: null,
    );
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
            source: state.source,
            accountName: state.accountName,
            identityDecisions: state.identityDecisions,
            reconciliationDecisions: state.reconciliationDecisions,
            confirmReactivation: state.confirmReactivation,
            mapping: mapping,
          );
      if (!mounted) return;
      state = state.copyWith(preview: preview, loading: false, error: null);
    } catch (error) {
      if (!mounted) return;
      state = state.copyWith(loading: false, error: error.toString());
    }
  }

  Future<void> setSource(String source, String account) async {
    if (state.loading) return;
    if (source.trim().isEmpty || account.trim().isEmpty) {
      state = state.copyWith(error: "取込元と口座名を入力してください。");
      return;
    }
    final mapping = state.preview?.mapping;
    state = state.copyWith(
      source: source.trim(),
      accountName: account.trim(),
      preview: null,
      commitResult: null,
      confirmReactivation: false,
      identityDecisions: [],
      reconciliationDecisions: [],
      error: null,
    );
    if (state.filePath != null) {
      state = state.copyWith(loading: true);
      await _preview(mapping);
    }
  }

  Future<void> resolve(List<ImportIdentityDecision> decisions) async {
    if (state.loading || state.preview == null) return;
    final mapping = state.preview!.mapping;
    state = state.copyWith(
      identityDecisions: List.unmodifiable(
        {
          for (final d in state.identityDecisions) d.row: d,
          for (final d in decisions) d.row: d,
        }.values,
      ),
      loading: true,
      error: null,
      commitResult: null,
    );
    await _preview(mapping);
  }

  Future<void> resolvePurchases(List<PurchaseCsvDecision> decisions) async {
    if (state.loading || state.preview == null) return;
    final mapping = state.preview!.mapping;
    state = state.copyWith(
      reconciliationDecisions: List.unmodifiable(
        {
          for (final d in state.reconciliationDecisions) d.row: d,
          for (final d in decisions) d.row: d,
        }.values,
      ),
      loading: true,
      error: null,
      commitResult: null,
    );
    await _preview(mapping);
  }

  Future<void> commit() async {
    if (state.loading) return;
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
      ref.invalidate(expenseCountProvider);
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
