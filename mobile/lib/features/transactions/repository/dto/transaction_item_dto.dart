import '../../../../shared/utils/json_parsing.dart';
import '../../domain/transaction_item.dart';

class TransactionItemDto {
  const TransactionItemDto({
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

  factory TransactionItemDto.fromJson(Map<String, dynamic> json) =>
      TransactionItemDto(
        id: parseInt(json['id']),
        transactionDate: json['transaction_date'] as String? ?? '',
        description: json['description'] as String? ?? '',
        normalizedMerchant: json['normalized_merchant'] as String? ?? '',
        amount: parseInt(json['amount']),
        transactionType: json['transaction_type'] as String? ?? 'unknown',
        category: json['category'] as String? ?? '未分類',
        source: json['source'] as String? ?? '',
        sourceAccountName: json['source_account_name'] as String? ?? '',
      );

  final int id;
  final String transactionDate;
  final String description;
  final String normalizedMerchant;
  final int amount;
  final String transactionType;
  final String category;
  final String source;
  final String sourceAccountName;

  TransactionItem toDomain() => TransactionItem(
    id: id,
    transactionDate: transactionDate,
    description: description,
    normalizedMerchant: normalizedMerchant,
    amount: amount,
    transactionType: transactionType,
    category: category,
    source: source,
    sourceAccountName: sourceAccountName,
  );
}
