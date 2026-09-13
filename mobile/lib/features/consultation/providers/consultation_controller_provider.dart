import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../screens/consultation/consultation_view_model.dart';
import '../domain/consultation.dart';

@riverpod
ConsultationViewModel consultationViewModelFactory(Ref ref) =>
    ConsultationViewModel(ref);

final consultationControllerProvider =
    StateNotifierProvider<ConsultationViewModel, AsyncValue<Consultation?>>(
      consultationViewModelFactory,
    );
