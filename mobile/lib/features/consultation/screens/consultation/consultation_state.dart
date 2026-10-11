import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/consultation.dart';
import '../../domain/chat_request.dart';

enum ConsultationPhase {
  idle,
  submitting,
  waiting,
  checking,
  cancelling,
  uncertain,
}

/// 相談の送信結果と履歴の読込状態を独立して保持する。
class ConsultationState {
  const ConsultationState({
    this.consultation = const AsyncData(null),
    this.history = const AsyncLoading(),
    this.phase = ConsultationPhase.idle,
    this.pendingRequest,
  });

  final ConsultationPhase phase;
  final PendingChatRequest? pendingRequest;
  bool get unresolved => pendingRequest != null;
  final AsyncValue<Consultation?> consultation;
  final AsyncValue<List<Consultation>> history;

  ConsultationState copyWith({
    AsyncValue<Consultation?>? consultation,
    AsyncValue<List<Consultation>>? history,
    ConsultationPhase? phase,
    PendingChatRequest? pendingRequest,
    bool clearPending = false,
  }) => ConsultationState(
    consultation: consultation ?? this.consultation,
    history: history ?? this.history,
    phase: phase ?? this.phase,
    pendingRequest: clearPending
        ? null
        : (pendingRequest ?? this.pendingRequest),
  );
}
