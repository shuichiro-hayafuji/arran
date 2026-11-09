class CsvMapping {
  const CsvMapping({
    this.transactionIdColumn = "",
    this.dateColumn = '',
    this.descriptionColumn = '',
    this.amountColumn = '',
    this.debitColumn = '',
    this.creditColumn = '',
  });

  final String transactionIdColumn;
  final String dateColumn;
  final String descriptionColumn;
  final String amountColumn;
  final String debitColumn;
  final String creditColumn;
}
