import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/consultation/domain/consultation.dart';
import 'package:spendable_today/features/consultation/repository/consultation_repository.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_intent.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_view_model.dart';
import 'package:spendable_today/features/consultation/screens/result/result_intent.dart';
import 'package:spendable_today/features/consultation/screens/result/result_view_model.dart';
import 'package:spendable_today/features/profile/repository/memory_repository.dart';

import 'widget_test.dart' show FakeConsultationRepository, FakeMemoryRepository, testConsultation;

void main() {
  late HistoryRepository repository;
  late ProviderContainer container;

  setUp(() {
    repository = HistoryRepository();
    container = ProviderContainer(overrides: [
      consultationRepositoryProvider.overrideWithValue(repository),
      memoryRepositoryProvider.overrideWithValue(FakeMemoryRepository()),
    ]);
  });

  tearDown(() => container.dispose());

  test('loads history without changing the submission state', () async {
    final vm = container.read(consultationControllerProvider.notifier);
    expect(vm.state.history.isLoading, isTrue);
    repository.requests.single.complete([testConsultation]);
    await Future<void>.delayed(Duration.zero);
    expect(vm.state.history.requireValue, [testConsultation]);
    expect(vm.state.consultation.requireValue, isNull);
  });

  test('submit and follow-up refresh history; refresh errors preserve advice', () async {
    final vm = container.read(consultationControllerProvider.notifier);
    repository.requests.single.complete([]);
    await Future<void>.delayed(Duration.zero);

    final submit = vm.dispatch(const SubmitConsultation('相談', 6000));
    await Future<void>.delayed(Duration.zero);
    repository.requests.last.completeError(StateError('history unavailable'));
    await submit;
    expect(vm.state.consultation.requireValue, testConsultation);
    expect(vm.state.history.hasError, isTrue);

    final followUp = vm.dispatch(const ContinueConsultation(1, '追加', 7000));
    await Future<void>.delayed(Duration.zero);
    repository.requests.last.complete([testConsultation]);
    await followUp;
    expect(repository.requests, hasLength(3));
    expect(vm.state.history.requireValue, [testConsultation]);
    await vm.dispatch(const ClearConsultation());
    expect(vm.state.consultation.requireValue, isNull);
    expect(vm.state.history.requireValue, [testConsultation]);
  });

  test('older history responses cannot overwrite a newer refresh', () async {
    final vm = container.read(consultationControllerProvider.notifier);
    final refresh = vm.refreshHistory();
    repository.requests.last.complete([testConsultation]);
    await refresh;
    repository.requests.first.complete([]);
    await Future<void>.delayed(Duration.zero);
    expect(vm.state.history.requireValue, [testConsultation]);
  });

  test('history completing after disposal is ignored', () async {
    final vm = container.read(consultationControllerProvider.notifier);
    final refresh = vm.refreshHistory();
    container.invalidate(consultationControllerProvider);
    for (final request in repository.requests) {
      request.complete([testConsultation]);
    }
    await refresh;
    await Future<void>.delayed(Duration.zero);
  });

  test('saving a result refreshes existing consultation history', () async {
    final vm = container.read(consultationControllerProvider.notifier);
    repository.requests.single.complete([testConsultation]);
    final result = container.read(resultViewModelProvider(1).notifier);
    repository.requests.last.complete([testConsultation]);
    await Future<void>.delayed(Duration.zero);
    await result.dispatch(const SaveResult(
      status: 'skipped', actualAmount: null, reason: '',
      satisfaction: 3, regret: 1, note: '',
    ));
    expect(result.state.saved, isTrue);
    expect(repository.requests, hasLength(3));
    repository.requests.last.complete([]);
    await Future<void>.delayed(Duration.zero);
    expect(vm.state.history.requireValue, isEmpty);
  });
}

class HistoryRepository extends FakeConsultationRepository {
  final requests = <Completer<List<Consultation>>>[];

  @override
  Future<List<Consultation>> getAll() {
    final request = Completer<List<Consultation>>();
    requests.add(request);
    return request.future;
  }
}
