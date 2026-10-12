import '../../../transactions/repository/reconciliation_repository.dart';
import 'dart:async';
import 'dart:math';
import '../../../../core/network/api_exception.dart';
import '../../../session/provider/session_provider.dart';
import '../../domain/chat_request.dart';
import '../../repository/pending_request_storage.dart';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:spendable_today/shared/presentation/mvi.dart';
import 'package:spendable_today/features/consultation/repository/consultation_repository.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_intent.dart';

import 'consultation_state.dart';
import '../../domain/consultation.dart';

@riverpod
ConsultationViewModel consultationViewModelFactory(Ref ref) =>
    ConsultationViewModel(ref);

final consultationControllerProvider =
    StateNotifierProvider<ConsultationViewModel, ConsultationState>(
      consultationViewModelFactory,
    );

/// 新規相談と追加回答を送信し、サーバーの判定結果を画面と履歴に反映する。
/// 予定額の未入力は null のまま送り、追加質問の要否はサーバーに委ねる。
class ConsultationViewModel
    extends MviViewModel<ConsultationState, ConsultationIntent> {
  ConsultationViewModel(this.ref) : super(const ConsultationState()) {
    _session = ref.read(sessionControllerProvider);
    _scope = _session.token;
    _generation = _session.generation;
    _storage = ref.read(pendingRequestStorageProvider);
    _restoration = _restore();
    unawaited(_restoration);
    unawaited(refreshHistory());
  }

  final Ref ref;
  int _historyRequestId = 0;
  late final SessionController _session;
  late final String? _scope;
  late final int _generation;
  late final PendingRequestStorage _storage;
  late final Future<void> _restoration;
  int _submissionGeneration = 0;
  bool _cancelRequested = false;
  bool get _active =>
      mounted && _session.generation == _generation && _session.token == _scope;
  static String _id() {
    final random = Random.secure();
    return List.generate(
      16,
      (_) => random.nextInt(256).toRadixString(16).padLeft(2, '0'),
    ).join();
  }

  @override
  Future<void> dispatch(ConsultationIntent intent) {
    switch (intent) {
      case SubmitConsultation(
        message: final message,
        plannedAmount: final amount,
      ):
        return submit(message, amount);
      case ContinueConsultation(
        id: final id,
        message: final message,
        plannedAmount: final amount,
      ):
        return continueWith(id, message, amount);
      case SelectConsultation(consultation: final item):
        select(item);
        return Future.value();
      case RecoverConsultationRequest():
        return recover();
      case CancelConsultationRequest():
        return cancelRequest();
      case ClearConsultation():
        clear();
        return Future.value();
    }
  }

  Future<void> submit(String message, int? amount) => _send(0, message, amount);
  Future<void> continueWith(int id, String message, int? amount) =>
      _send(id, message, amount);
  Future<void> _restore() async {
    try {
      final pending = await _storage.load(_scope);
      if (!_active ||
          pending == null ||
          state.pendingRequest != null ||
          state.consultation.isLoading)
        return;
      _cancelRequested = pending.cancelRequested;
      state = state.copyWith(
        pendingRequest: pending,
        phase: ConsultationPhase.uncertain,
      );
    } catch (error, stack) {
      if (_active)
        state = state.copyWith(
          consultation: AsyncError<Consultation?>(
            const ApiException('送信内容を復元できませんでした。再度入力してください。'),
            stack,
          ),
        );
    }
  }

  Future<void> _send(int id, String message, int? amount) async {
    await _restoration;
    if (!_active || state.consultation.isLoading) return;
    if (state.pendingRequest != null) {
      await recover();
      return;
    }
    final pending = PendingChatRequest(
      id: _id(),
      message: message.trim(),
      consultationId: id,
      plannedAmount: amount,
    );
    _cancelRequested = false;
    state = state.copyWith(pendingRequest: pending);
    await _run(pending, submit: true);
  }

  Future<void> recover() async {
    if (!_active || state.consultation.isLoading) return;
    final pending = state.pendingRequest;
    if (pending == null) return;
    await _run(pending, submit: false);
  }

  Future<void> cancelRequest() async {
    final pending = state.pendingRequest;
    if (!_active || pending == null) return;
    _cancelRequested = true;
    final cancelled = pending.withCancellation();
    state = state.copyWith(
      pendingRequest: cancelled,
      phase: ConsultationPhase.cancelling,
    );
    try {
      await _storage.save(_scope, cancelled);
    } catch (_) {
      if (_active) state = state.copyWith(phase: ConsultationPhase.uncertain);
      return;
    }
    if (!_active) return;
    if (!state.consultation.isLoading) {
      await _run(cancelled, submit: false);
      return;
    }
    // Wake the server while the bounded result loop remains the only result publisher.
    try {
      await ref.read(consultationRepositoryProvider).cancelRequest(pending.id);
    } catch (_) {}
  }

  Future<void> _run(PendingChatRequest pending, {required bool submit}) async {
    final operation = ++_submissionGeneration;
    final previous = state.consultation;
    state = state.copyWith(
      consultation: const AsyncLoading<Consultation?>().copyWithPrevious(
        previous,
      ),
      phase: submit ? ConsultationPhase.submitting : ConsultationPhase.checking,
    );
    final budget = Stopwatch()..start();
    bool valid() => _active && operation == _submissionGeneration;
    Duration remaining() {
      final value = const Duration(seconds: 45) - budget.elapsed;
      if (value <= Duration.zero)
        throw const ApiException('保存結果をまだ確認できません。入力を保持しています。「結果を確認」で再確認できます。');
      return value;
    }

    try {
      await _storage.save(_scope, pending).timeout(remaining());
      if (!valid()) return;
      final repository = ref.read(consultationRepositoryProvider);
      Future<ChatRequestReceipt> accept() async {
        try {
          return await repository.submitRequest(pending).timeout(remaining());
        } catch (error) {
          if (error is ApiException &&
              (error.statusCode == 400 || error.statusCode == 404)) {
            await _storage.clear(_scope).timeout(remaining());
            if (valid())
              state = state.copyWith(
                clearPending: true,
                phase: ConsultationPhase.idle,
              );
          }
          rethrow;
        }
      }

      ChatRequestReceipt? receipt;
      if (submit && !_cancelRequested) {
        try {
          receipt = await accept();
        } catch (error) {
          if (error is ApiException &&
              error.statusCode != null &&
              error.statusCode! >= 400) {
            rethrow;
          }
        }
      }
      for (var attempt = 0; attempt < 24; attempt++) {
        if (!valid()) return;
        if (receipt != null && receipt.id != pending.id)
          throw const ApiException('別の送信結果が返されました。保存結果を再確認してください。');
        if (receipt?.terminal == true) {
          if (receipt!.result != null) {
            await _storage.clear(_scope).timeout(remaining());
            if (!valid()) return;
            state = state.copyWith(
              consultation: AsyncData(receipt.result),
              phase: ConsultationPhase.idle,
              clearPending: true,
            );
            ref.invalidate(expenseCountProvider);
            await refreshHistory();
            return;
          }
          await _storage.clear(_scope).timeout(remaining());
          if (!valid()) return;
          state = state.copyWith(
            phase: ConsultationPhase.idle,
            clearPending: true,
          );
          throw const ApiException('保存できませんでした。入力を保持しています。もう一度送信できます。');
        }
        state = state.copyWith(
          phase: _cancelRequested
              ? ConsultationPhase.cancelling
              : ConsultationPhase.waiting,
        );
        try {
          if (_cancelRequested) {
            receipt = await repository
                .cancelRequest(pending.id)
                .timeout(remaining());
          } else {
            receipt = await repository
                .requestResult(pending.id)
                .timeout(remaining());
          }
        } catch (error) {
          if (error is ApiException && error.statusCode == 404) {
            if (_cancelRequested) {
              throw const ApiException("中止をまだ確認できません。結果を再確認してください。");
            }
            receipt = await accept();
          } else if (error is ApiException &&
              (error.statusCode == 401 || error.statusCode == 403)) {
            rethrow;
          } else {
            receipt = null;
          }
        }
        if (receipt?.terminal == true) continue;
        final pause = Duration(seconds: attempt < 2 ? 1 : 2);
        await Future<void>.delayed(pause < remaining() ? pause : remaining());
      }
      throw const ApiException('保存結果をまだ確認できません。入力を保持しています。「結果を確認」で再確認できます。');
    } catch (error, stack) {
      if (!valid()) return;
      state = state.copyWith(
        consultation: AsyncError<Consultation?>(
          error,
          stack,
        ).copyWithPrevious(previous),
        phase: state.pendingRequest == null
            ? ConsultationPhase.idle
            : ConsultationPhase.uncertain,
      );
    }
  }

  @override
  void dispose() {
    _submissionGeneration++;
    if (_session.token != _scope)
      unawaited(_storage.clear(_scope).catchError((Object _) {}));
    super.dispose();
  }

  /// 初期表示と相談・結果の保存後に履歴を再取得する。
  /// 遅れて返った古いリクエストで最新の履歴を上書きしない。
  Future<void> refreshHistory() async {
    final requestId = ++_historyRequestId;
    state = state.copyWith(history: const AsyncLoading());
    try {
      final items = await ref.read(consultationRepositoryProvider).getAll();
      if (!_active || requestId != _historyRequestId) return;
      state = state.copyWith(history: AsyncData(List.unmodifiable(items)));
    } catch (error, stackTrace) {
      if (!_active || requestId != _historyRequestId) return;
      state = state.copyWith(history: AsyncError(error, stackTrace));
    }
  }

  void select(Consultation item) {
    if (state.consultation.isLoading || state.unresolved) return;
    state = state.copyWith(consultation: AsyncData(item));
  }

  void clear() {
    if (state.consultation.isLoading || state.unresolved) return;
    state = state.copyWith(consultation: const AsyncData(null));
  }
}
