import 'package:flutter/material.dart';
import '../history/management_screen.dart';

class TransactionDetailScreen extends StatelessWidget {
  const TransactionDetailScreen({required this.transactionId, super.key});
  final int transactionId;
  @override
  Widget build(BuildContext context) =>
      ManagementScreen(id: transactionId.toString(), isImport: false);
}
