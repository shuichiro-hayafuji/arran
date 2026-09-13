import 'csv_mapping.dart';
import 'transaction_item.dart';

class ImportPreview {
  const ImportPreview({
    required this.previewId,
    required this.detectedEncoding,
    required this.headers,
    required this.mapping,
    required this.needsMapping,
    required this.readCount,
    required this.periodStart,
    required this.periodEnd,
    required this.expenseTotal,
    required this.rows,
    required this.warnings,
  });

  final String previewId;
  final String detectedEncoding;
  final List<String> headers;
  final CsvMapping mapping;
  final bool needsMapping;
  final int readCount;
  final String periodStart;
  final String periodEnd;
  final int expenseTotal;
  final List<TransactionItem> rows;
  final List<String> warnings;
}
