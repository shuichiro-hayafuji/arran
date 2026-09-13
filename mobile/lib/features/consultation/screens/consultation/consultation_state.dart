import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/consultation.dart';

/// 相談の送信結果と履歴の読込状態を独立して保持する。
class ConsultationState {
  const ConsultationState({
    this.consultation = const AsyncData(null),
    this.history = const AsyncLoading(),
  });

  final AsyncValue<Consultation?> consultation;
  final AsyncValue<List<Consultation>> history;

  ConsultationState copyWith({
    AsyncValue<Consultation?>? consultation,
    AsyncValue<List<Consultation>>? history,
  }) => ConsultationState(
    consultation: consultation ?? this.consultation,
    history: history ?? this.history,
  );
}
