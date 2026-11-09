class ExpenseCount {
  const ExpenseCount({
    required this.confirmed,
    required this.estimated,
    required this.undated,
    required this.unresolved,
    required this.pendingRecalculation,
  });
  final int confirmed;
  final int estimated;
  final int undated;
  final int unresolved;
  final int pendingRecalculation;
}
