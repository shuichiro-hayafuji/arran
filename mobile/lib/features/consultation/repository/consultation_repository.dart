import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';
import '../domain/consultation.dart';
import 'dto/consultation_dto.dart';

@riverpod
ConsultationRepository consultationRepository(Ref ref) =>
    ConsultationRepositoryImpl(ref.watch(authorizedApiClientProvider));

final consultationRepositoryProvider = Provider<ConsultationRepository>(
  consultationRepository,
);

abstract interface class ConsultationRepository {
  Future<Consultation> start(String message, int? plannedAmount);
  Future<Consultation> continueConversation(
    int id,
    String message,
    int? plannedAmount,
  );
  Future<List<Consultation>> getAll();
  Future<Consultation> updateResult(
    int id, {
    required String status,
    int? actualAmount,
    String reason,
    int? satisfaction,
    int? regret,
    String note,
  });
}

class ConsultationRepositoryImpl implements ConsultationRepository {
  const ConsultationRepositoryImpl(this._client);

  final ApiClient _client;

  @override
  Future<Consultation> start(String message, int? plannedAmount) async {
    final dto = StartConsultationRequestDto(message, plannedAmount);
    final response = await _client.post('/consultations', data: dto.toJson());
    return ConsultationDto.fromJson(parseMap(response.data)).toDomain();
  }

  @override
  Future<Consultation> continueConversation(
    int id,
    String message,
    int? plannedAmount,
  ) async {
    final dto = ContinueConsultationRequestDto(message, plannedAmount);
    final response = await _client.post(
      '/consultations/$id/messages',
      data: dto.toJson(),
    );
    return ConsultationDto.fromJson(parseMap(response.data)).toDomain();
  }

  @override
  Future<List<Consultation>> getAll() async {
    final response = await _client.get('/consultations');
    return parseMaps(
      parseMap(response.data)['items'],
    ).map(ConsultationDto.fromJson).map((dto) => dto.toDomain()).toList();
  }

  @override
  Future<Consultation> updateResult(
    int id, {
    required String status,
    int? actualAmount,
    String reason = '',
    int? satisfaction,
    int? regret,
    String note = '',
  }) async {
    final dto = UpdateConsultationResultRequestDto(
      status: status,
      actualAmount: actualAmount,
      reason: reason,
      satisfaction: satisfaction,
      regret: regret,
      note: note,
    );
    final response = await _client.patch(
      '/consultations/$id/result',
      data: dto.toJson(),
    );
    return ConsultationDto.fromJson(parseMap(response.data)).toDomain();
  }
}
