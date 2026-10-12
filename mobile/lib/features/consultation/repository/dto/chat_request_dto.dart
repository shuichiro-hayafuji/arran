import '../../domain/chat_request.dart';
import 'consultation_dto.dart';

class ChatRequestDto {
  static Map<String, dynamic> request(PendingChatRequest value) => {
    'consultation_id': value.consultationId,
    'message': value.message,
    'planned_amount': value.plannedAmount,
  };
  static ChatRequestReceipt receipt(Object? value) {
    if (value is! Map<String, dynamic> ||
        value['request_id'] is! String ||
        value['status'] is! String ||
        value['expires_at'] is! String ||
        value['cancel_requested'] is! bool ||
        value['error_code'] is! String) {
      throw const FormatException('相談の受付情報が不正です');
    }
    final status = value['status'] as String;
    if (!{'processing', 'completed', 'cancelled', 'failed'}.contains(status)) {
      throw const FormatException('相談の受付状態が不正です');
    }
    final result = value['result'];
    if (status == 'completed' && result is! Map<String, dynamic>) {
      throw const FormatException('保存結果がありません');
    }
    return ChatRequestReceipt(
      id: value['request_id'] as String,
      status: status,
      expiresAt: DateTime.parse(value['expires_at'] as String),
      cancelRequested: value['cancel_requested'] as bool,
      errorCode: value['error_code'] as String,
      result: result is Map<String, dynamic>
          ? ConsultationDto.fromJson(result).toDomain()
          : null,
    );
  }
}
