import 'consultation.dart';

class PendingChatRequest {
  const PendingChatRequest({
    required this.id,
    required this.message,
    this.consultationId = 0,
    this.plannedAmount,
    this.cancelRequested = false,
  });
  final String id;
  final String message;
  final int consultationId;
  final int? plannedAmount;
  final bool cancelRequested;
  PendingChatRequest withCancellation() => PendingChatRequest(
    id: id,
    message: message,
    consultationId: consultationId,
    plannedAmount: plannedAmount,
    cancelRequested: true,
  );
}

class ChatRequestReceipt {
  const ChatRequestReceipt({
    required this.id,
    required this.status,
    required this.expiresAt,
    this.cancelRequested = false,
    this.errorCode = '',
    this.result,
  });
  final String id;
  final String status;
  final DateTime expiresAt;
  final bool cancelRequested;
  final String errorCode;
  final Consultation? result;
  bool get terminal => status != 'processing';
}
