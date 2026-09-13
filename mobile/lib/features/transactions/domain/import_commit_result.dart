class ImportCommitResult {
  const ImportCommitResult({
    required this.importedCount,
    required this.duplicateCount,
  });

  final int importedCount;
  final int duplicateCount;
}
