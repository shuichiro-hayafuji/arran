import 'dart:async';

import '../../repository/reconciliation_repository.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/core/network/api_exception.dart';
import 'package:spendable_today/features/session/provider/session_provider.dart';
import 'package:spendable_today/features/transactions/repository/pending_import_storage.dart';
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
    @Default(false) bool commitOutcomeUnknown,
    String? pendingPreviewId,
    @Default(true) bool restoring,
    @Default(false) bool recoveryBlocked,
    @Default(false) bool cleanupRejected,
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
  bool get commitOutcomeUnknown;
  String? get pendingPreviewId;
  bool get restoring;
  bool get recoveryBlocked;
  bool get cleanupRejected;
  bool get loading;
  String? get error;
}

/// CSV の選択・列の再割当・プレビュー確認・確定を管理する。
/// 確定にはサーバー発行の previewId を使い、成功後に明細と月次集計を再取得する。
class ImportViewModel extends MviViewModel<ImportState, ImportIntent> {
  ImportViewModel(this.ref) : super(const ImportState()) {
    _session = ref.read(sessionControllerProvider);
    _token = _session.token;
    _generation = _session.generation;
    _resolveOwner = ref.read(pendingImportOwnerResolverProvider);
    _storage = ref.read(pendingImportStorageProvider);
    _ready = _restore();
  }

  final Ref ref;
  late final SessionController _session;
  late final String? _token;
  late final int _generation;
  late final PendingImportOwnerResolver _resolveOwner;
  int? _ownerId;
  late final PendingImportStorage _storage;
  late Future<void> _ready;

  bool get _isCurrentSession =>
      _session.token == _token && _session.generation == _generation;

  Future<void> _restore() async {
    try {
      final owner = await _resolveOwner();
      if (!mounted) return;
      if (!_isCurrentSession || owner <= 0) {
        throw StateError('Session changed or owner unavailable');
      }
      _ownerId = owner;
      final id = await _storage.load(owner);
      if (!mounted) return;
      if (!_isCurrentSession) throw StateError('Session changed');
      state = state.copyWith(
        pendingPreviewId: id,
        commitOutcomeUnknown: id != null,
        restoring: false,
        recoveryBlocked: false,
        error: id == null ? null : '前回の保存結果を確認してください。',
      );
    } catch (_) {
      if (!mounted) return;
      state = state.copyWith(
        restoring: false,
        recoveryBlocked: true,
        error: '利用者または保存結果の確認情報を確認できません。読み直してから操作してください。',
      );
    }
  }

  Future<void> retryRestore() async {
    if (state.restoring || !state.recoveryBlocked) return;
    state = state.copyWith(restoring: true, error: null);
    _ready = _restore();
    await _ready;
  }

  bool get _blocked =>
      state.loading ||
      state.restoring ||
      state.recoveryBlocked ||
      state.cleanupRejected ||
      state.commitOutcomeUnknown ||
      state.commitResult != null;

  Future<void> retryRejectedCleanup() async {
    final id = state.pendingPreviewId;
    if (!state.cleanupRejected || state.loading || id == null) return;
    state = state.copyWith(loading: true, error: null);
    try {
      if (!_isCurrentSession) throw StateError('Session changed');
      await _storage.clear(_ownerId, id);
      if (!mounted) return;
      state = state.copyWith(
        loading: false,
        cleanupRejected: false,
        pendingPreviewId: null,
        error: '保存は拒否されました。内容を確認して再度プレビューしてください。',
      );
    } catch (_) {
      if (!mounted) return;
      state = state.copyWith(
        loading: false,
        error: '端末の確認情報を消せません。もう一度お試しください。',
      );
    }
  }

  Future<void> resumeAfterOperatorConfirmation() async {
    final id = state.pendingPreviewId;
    if (!state.commitOutcomeUnknown ||
        state.loading ||
        state.restoring ||
        state.recoveryBlocked ||
        id == null ||
        !_isCurrentSession)
      return;
    state = state.copyWith(loading: true, error: null);
    try {
      await _storage.clear(_ownerId, id);
      if (!mounted) return;
      if (!_isCurrentSession) throw StateError('Session changed');
      state = const ImportState(restoring: false);
    } catch (_) {
      if (!mounted) return;
      state = state.copyWith(
        loading: false,
        error: '確認情報を削除できません。もう一度お試しください。',
      );
    }
  }

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
        if (_blocked || state.preview == null) {
          return Future.value();
        }
        state = state.copyWith(
          confirmReactivation: true,
          loading: true,
          error: null,
        );
        return _preview(state.preview!.mapping);
      case CommitImport():
        return commit();
      case ResetImport():
        return reset();
    }
  }

  Future<void> selectAndPreview({
    required String filePath,
    required String fileName,
    CsvMapping? mapping,
  }) async {
    await _ready;
    if (!mounted || _blocked) return;
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
    await _ready;
    if (!mounted || _blocked) return;
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
      if (!_isCurrentSession) throw StateError('Session changed');
      final preview = await ref
          .read(transactionsRepositoryProvider)
          .previewImport(
            sessionGeneration: _generation,
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
      if (!_isCurrentSession) throw StateError('Session changed');
      state = state.copyWith(preview: preview, loading: false, error: null);
    } catch (error) {
      if (!mounted) return;
      if (!_isCurrentSession) {
        state = state.copyWith(
          loading: false,
          recoveryBlocked: true,
          error: 'ログイン状態が変わりました。もう一度ログインしてください。',
        );
        return;
      }
      state = state.copyWith(loading: false, error: error.toString());
    }
  }

  Future<void> setSource(String source, String account) async {
    await _ready;
    if (!mounted || _blocked) return;
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
    await _ready;
    if (!mounted || _blocked || state.preview == null) return;
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
    await _ready;
    if (!mounted || _blocked || state.preview == null) return;
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
    await _ready;
    if (!mounted ||
        state.loading ||
        state.restoring ||
        state.recoveryBlocked ||
        state.cleanupRejected ||
        state.commitResult != null)
      return;
    if (!_isCurrentSession) {
      state = state.copyWith(
        recoveryBlocked: true,
        error: 'ログイン状態が変わりました。もう一度ログインしてください。',
      );
      return;
    }
    final id = state.pendingPreviewId ?? state.preview?.previewId;
    if (id == null || id.isEmpty) {
      state = state.copyWith(error: '有効なプレビューがありません。');
      return;
    }
    final previouslyUnknown = state.commitOutcomeUnknown;
    // From the moment the request starts its outcome can be unknown on exit.
    state = state.copyWith(
      loading: true,
      commitOutcomeUnknown: true,
      error: null,
    );
    if (state.pendingPreviewId == null) {
      try {
        await _storage.save(_ownerId, id);
        if (!mounted) return;
        if (!_isCurrentSession) {
          state = state.copyWith(
            loading: false,
            recoveryBlocked: true,
            error: 'ログイン状態が変わったため、取込は開始していません。',
          );
          return;
        }
        state = state.copyWith(pendingPreviewId: id);
      } catch (_) {
        if (!mounted) return;
        state = state.copyWith(
          loading: false,
          commitOutcomeUnknown: false,
          error: '確認情報を端末に保存できないため、取込は開始していません。',
        );
        return;
      }
    }
    late final ImportCommitResult result;
    try {
      result = await ref
          .read(transactionsRepositoryProvider)
          .commitImport(id, sessionGeneration: _generation);
    } catch (error) {
      if (!mounted) return;
      if (!_isCurrentSession) {
        state = state.copyWith(
          loading: false,
          recoveryBlocked: true,
          error: 'ログイン状態が変わりました。保存結果を確認できません。',
        );
        return;
      }
      // A later rejection cannot prove that an earlier attempt did not commit.
      final status = error is ApiException ? error.statusCode : null;
      final definitelyRejected =
          status == 400 ||
          status == 401 ||
          status == 403 ||
          status == 404 ||
          status == 409 ||
          status == 422;
      final unknown = previouslyUnknown || !definitelyRejected;
      if (!unknown) {
        try {
          if (!_isCurrentSession) throw StateError('Session changed');
          await _storage.clear(_ownerId, id);
        } catch (_) {
          if (!mounted) return;
          state = state.copyWith(
            loading: false,
            commitOutcomeUnknown: false,
            cleanupRejected: true,
            error: '保存は拒否されましたが、端末の確認情報を消せません。削除をもう一度お試しください。',
          );
          return;
        }
      }
      if (!mounted) return;
      state = state.copyWith(
        loading: false,
        commitOutcomeUnknown: unknown,
        pendingPreviewId: unknown ? id : null,
        error: unknown
            ? '保存できたか確認できません。同じ内容で保存結果を確認してください。'
            : error is ApiException
            ? error.message
            : '保存できませんでした。',
      );
      return;
    }
    if (!mounted) return;
    if (!_isCurrentSession) {
      state = state.copyWith(
        loading: false,
        recoveryBlocked: true,
        error: 'ログイン状態が変わりました。保存結果を確認できません。',
      );
      return;
    }
    state = state.copyWith(
      commitResult: result,
      commitOutcomeUnknown: false,
      pendingPreviewId: id,
      loading: false,
      error: null,
    );
    try {
      ref.invalidate(expenseCountProvider);
      unawaited(
        Future.wait([
          ref
              .read(transactionsViewModelProvider.notifier)
              .dispatch(const RefreshTransactions()),
          ref
              .read(dashboardViewModelProvider.notifier)
              .dispatch(const RefreshDashboard()),
        ]).then<void>(
          (_) {
            if (!mounted) return;
            if (!_isCurrentSession) {
              _refreshSessionChanged(result);
              return;
            }
            if (ref.read(transactionsViewModelProvider).data.hasError ||
                ref.read(dashboardViewModelProvider).data.hasError) {
              _refreshFailed(result);
            }
          },
          onError: (Object _, StackTrace __) {
            _refreshFailed(result);
          },
        ),
      );
    } catch (_) {
      _refreshFailed(result);
    }
  }

  void _refreshSessionChanged(ImportCommitResult result) {
    if (mounted && identical(state.commitResult, result)) {
      state = state.copyWith(
        commitResult: null,
        commitOutcomeUnknown: true,
        recoveryBlocked: true,
        error: 'ログイン状態が変わりました。保存結果を確認できません。',
      );
    }
  }

  void _refreshFailed(ImportCommitResult result) {
    if (!_isCurrentSession) {
      _refreshSessionChanged(result);
      return;
    }
    if (mounted && identical(state.commitResult, result)) {
      state = state.copyWith(error: '保存は完了しましたが、一覧を更新できませんでした。後で読み直してください。');
    }
  }

  Future<void> reset() async {
    if (state.loading ||
        state.restoring ||
        state.recoveryBlocked ||
        state.cleanupRejected ||
        state.commitOutcomeUnknown)
      return;
    final id = state.pendingPreviewId;
    if (id != null) {
      state = state.copyWith(loading: true, error: null);
      try {
        if (!_isCurrentSession) throw StateError('Session changed');
        await _storage.clear(_ownerId, id);
      } catch (_) {
        if (!mounted) return;
        state = state.copyWith(
          loading: false,
          error: '端末の確認情報を消せません。もう一度お試しください。',
        );
        return;
      }
      if (!mounted) return;
    }
    state = const ImportState();
    state = state.copyWith(restoring: false);
  }
}
