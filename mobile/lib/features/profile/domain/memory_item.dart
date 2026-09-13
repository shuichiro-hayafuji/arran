class MemoryItem {
  const MemoryItem({
    required this.id,
    required this.type,
    required this.content,
    required this.evidence,
    required this.confidence,
    this.createdAt = '',
    this.updatedAt = '',
  });

  final int id;
  final String type;
  final String content;
  final String evidence;
  final double confidence;
  final String createdAt;
  final String updatedAt;
}
