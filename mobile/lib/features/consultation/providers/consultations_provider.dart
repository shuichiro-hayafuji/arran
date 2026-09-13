import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../domain/consultation.dart';
import 'consultation_repository_provider.dart';

@riverpod
Future<List<Consultation>> consultations(Ref ref) =>
    ref.read(consultationRepositoryProvider).getAll();

final consultationsProvider = FutureProvider<List<Consultation>>(consultations);
