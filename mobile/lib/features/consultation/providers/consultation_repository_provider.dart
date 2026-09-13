import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../repository/consultation_repository.dart';

@riverpod
ConsultationRepository consultationRepository(Ref ref) =>
    ConsultationRepositoryImpl(ref.watch(authorizedApiClientProvider));

final consultationRepositoryProvider = Provider<ConsultationRepository>(
  consultationRepository,
);
