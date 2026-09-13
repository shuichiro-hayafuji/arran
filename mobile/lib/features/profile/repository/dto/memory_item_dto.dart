import '../../../../shared/utils/json_parsing.dart';
import '../../domain/memory_item.dart';

class MemoryItemDto {
  const MemoryItemDto({
    required this.id,
    required this.type,
    required this.content,
    required this.evidence,
    required this.confidence,
    required this.createdAt,
    required this.updatedAt,
  });

  factory MemoryItemDto.fromJson(Map<String, dynamic> json) => MemoryItemDto(
    id: parseInt(json['id']),
    type: json['type'] as String? ?? '',
    content: json['content'] as String? ?? '',
    evidence: json['evidence'] as String? ?? '',
    confidence: (json['confidence'] as num?)?.toDouble() ?? 0,
    createdAt: json['created_at'] as String? ?? '',
    updatedAt: json['updated_at'] as String? ?? '',
  );

  factory MemoryItemDto.fromDomain(MemoryItem item) => MemoryItemDto(
    id: item.id,
    type: item.type,
    content: item.content,
    evidence: item.evidence,
    confidence: item.confidence,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
  );

  final int id;
  final String type;
  final String content;
  final String evidence;
  final double confidence;
  final String createdAt;
  final String updatedAt;

  Map<String, dynamic> toJson() => {
    'id': id,
    'type': type,
    'content': content,
    'evidence': evidence,
    'confidence': confidence,
  };

  MemoryItem toDomain() => MemoryItem(
    id: id,
    type: type,
    content: content,
    evidence: evidence,
    confidence: confidence,
    createdAt: createdAt,
    updatedAt: updatedAt,
  );
}
