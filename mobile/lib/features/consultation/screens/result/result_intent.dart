import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/shared/presentation/mvi.dart';

part 'result_intent.freezed.dart';

@freezed
sealed class ResultIntent with _$ResultIntent implements MviIntent {
  const ResultIntent._();

  const factory ResultIntent.load() = LoadResult;
  const factory ResultIntent.save({
    required String status,
    required int? actualAmount,
    required String reason,
    required int satisfaction,
    required int regret,
    required String note,
  }) = SaveResult;
}
