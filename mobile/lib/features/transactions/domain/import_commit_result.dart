class ImportCommitResult {
  const ImportCommitResult({
    this.importId = "",
    this.reactivatedCount = 0,
    this.financialPending = 0,
    required this.importedCount,
    required this.duplicateCount,
  });

  final String importId;
  final int reactivatedCount, financialPending;
  final int importedCount;
  final int duplicateCount;
}
