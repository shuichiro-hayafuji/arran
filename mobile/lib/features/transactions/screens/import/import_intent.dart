import 'package:freezed_annotation/freezed_annotation.dart';

import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/shared/presentation/mvi.dart';

part 'import_intent.freezed.dart';

@freezed
sealed class ImportIntent with _$ImportIntent implements MviIntent {
  const ImportIntent._();

  const factory ImportIntent.selectFile({
    required String filePath,
    required String fileName,
  }) = SelectImportFile;
  const factory ImportIntent.applyMapping(CsvMapping mapping) =
      ApplyImportMapping;
  const factory ImportIntent.commit() = CommitImport;
  const factory ImportIntent.reset() = ResetImport;
}
