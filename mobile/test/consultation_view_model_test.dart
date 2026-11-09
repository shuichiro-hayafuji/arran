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
import 'package:spendable_today/features/startup/screens/startup/startup_view_model.dart';

import 'widget_test.dart'
    show FakeConsultationRepository, FakeMemoryRepository, testConsultation;

void main() {
  late HistoryRepository repository;
  late ProviderContainer container;

  setUp(() {
    repository = HistoryRepository();
    container = ProviderContainer(
      overrides: [
        consultationRepositoryProvider.overrideWithValue(repository),
        memoryRepositoryProvider.overrideWithValue(FakeMemoryRepository()),
      ],
    );
  });

  tearDown(() => container.dispose());

  test('loads history without changing the submission state', () async {
    container.read(consultationControllerProvider.notifier);
    expect(
      container.read(consultationControllerProvider).history.isLoading,
      isTrue,
    );
    repository.requests.single.complete([testConsultation]);
    await Future<void>.delayed(Duration.zero);
    expect(
      container.read(consultationControllerProvider).history.requireValue,
      [testConsultation],
    );
    expect(
      container.read(consultationControllerProvider).consultation.requireValue,
      isNull,
    );
  });

  test(
    'submit and follow-up refresh history; refresh errors preserve advice',
    () async {
      final vm = container.read(consultationControllerProvider.notifier);
      repository.requests.single.complete([]);
      await Future<void>.delayed(Duration.zero);

      final submit = vm.dispatch(const SubmitConsultation('相談', 6000));
      await Future<void>.delayed(Duration.zero);
      repository.requests.last.completeError(StateError('history unavailable'));
      await submit;
      expect(
        container
            .read(consultationControllerProvider)
            .consultation
            .requireValue,
        testConsultation,
      );
      expect(
        container.read(consultationControllerProvider).history.hasError,
        isTrue,
      );

      final followUp = vm.dispatch(const ContinueConsultation(1, '追加', 7000));
      await Future<void>.delayed(Duration.zero);
      repository.requests.last.complete([testConsultation]);
      await followUp;
      expect(repository.requests, hasLength(3));
      expect(
        container.read(consultationControllerProvider).history.requireValue,
        [testConsultation],
      );
      await vm.dispatch(const ClearConsultation());
      expect(
        container
            .read(consultationControllerProvider)
            .consultation
            .requireValue,
        isNull,
      );
      expect(
        container.read(consultationControllerProvider).history.requireValue,
        [testConsultation],
      );
    },
  );

  test('older history responses cannot overwrite a newer refresh', () async {
    final vm = container.read(consultationControllerProvider.notifier);
    final refresh = vm.refreshHistory();
    repository.requests.last.complete([testConsultation]);
    await refresh;
    repository.requests.first.complete([]);
    await Future<void>.delayed(Duration.zero);
    expect(
      container.read(consultationControllerProvider).history.requireValue,
      [testConsultation],
    );
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
    container.read(consultationControllerProvider.notifier);
    repository.requests.single.complete([testConsultation]);
    final result = container.read(resultViewModelProvider(1).notifier);
    repository.requests.last.complete([testConsultation]);
    await Future<void>.delayed(Duration.zero);
    await result.dispatch(
      const SaveResult(
        status: 'skipped',
        actualAmount: null,
        reason: '',
        satisfaction: 3,
        regret: 1,
        note: '',
      ),
    );
    expect(container.read(resultViewModelProvider(1)).saved, isTrue);
    expect(repository.requests, hasLength(3));
    repository.requests.last.complete([]);
    await Future<void>.delayed(Duration.zero);
    expect(
      container.read(consultationControllerProvider).history.requireValue,
      isEmpty,
    );
  });
  test('startup opens chat without a profile request', () {
    final chatContainer = ProviderContainer();
    addTearDown(chatContainer.dispose);
    expect(
      chatContainer.read(startupViewModelProvider).decision.requireValue!.path,
      '/consult',
    );
  });

  test('duplicate submission is blocked and failure allows retry', () async {
    final delayed = DelayedRepository();
    final chatContainer = ProviderContainer(
      overrides: [consultationRepositoryProvider.overrideWithValue(delayed)],
    );
    addTearDown(chatContainer.dispose);
    final vm = chatContainer.read(consultationControllerProvider.notifier);
    final pending = vm.dispatch(const SubmitConsultation('会話だけで相談', null));
    await vm.dispatch(const ClearConsultation());
    expect(
      chatContainer.read(consultationControllerProvider).consultation.isLoading,
      isTrue,
    );
    await vm.dispatch(const SubmitConsultation('同じ相談', null));
    expect(delayed.calls, 1);
    delayed.response.completeError(StateError('offline'));
    await pending;
    expect(
      chatContainer.read(consultationControllerProvider).consultation.hasError,
      isTrue,
    );
    delayed.response = Completer<Consultation>();
    final retry = vm.dispatch(const SubmitConsultation('会話だけで相談', null));
    delayed.response.complete(testConsultation);
    await retry;
    expect(delayed.calls, 2);
    expect(
      chatContainer
          .read(consultationControllerProvider)
          .consultation
          .requireValue,
      testConsultation,
    );
  });
}

class DelayedRepository extends FakeConsultationRepository {
  int calls = 0;
  Completer<Consultation> response = Completer<Consultation>();
  @override
  Future<Consultation> start(String message, int? amount) {
    calls++;
    return response.future;
  }

  @override
  Future<List<Consultation>> getAll() async => [];
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
