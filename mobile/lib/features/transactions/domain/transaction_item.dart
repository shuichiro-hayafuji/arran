class TransactionItem {
  const TransactionItem({
    required this.id,
    required this.transactionDate,
    required this.description,
    required this.normalizedMerchant,
    required this.amount,
    required this.transactionType,
    required this.category,
    required this.source,
    required this.sourceAccountName,
  });

  final int id;
  final String transactionDate;
  final String description;
  final String normalizedMerchant;
  final int amount;
  final String transactionType;
  final String category;
  final String source;
  final String sourceAccountName;
}
