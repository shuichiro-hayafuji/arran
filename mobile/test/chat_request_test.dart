import 'dart:async';
import 'package:spendable_today/features/session/provider/session_provider.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/core/network/api_exception.dart';
import 'package:spendable_today/features/consultation/domain/chat_request.dart';
import 'package:spendable_today/features/consultation/repository/consultation_repository.dart';
import 'package:spendable_today/features/consultation/repository/pending_request_storage.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_screen.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_view_model.dart';
import 'widget_test.dart' show FakeConsultationRepository;

class RequestRepository extends FakeConsultationRepository {
  final submitted = <PendingChatRequest>[];
  int reads = 0;
  int cancellations = 0;
  bool lostAcceptance = false;
  int? rejectCode;
  bool missingCancellation = false;
  final hold = Completer<void>();
  bool delayed = false;
  bool processing = false;
  @override
  Future<ChatRequestReceipt> submitRequest(PendingChatRequest request) async {
    submitted.add(request);
    if (rejectCode != null)
      throw ApiException("invalid", statusCode: rejectCode);
    if (lostAcceptance && submitted.length == 1) throw StateError('wire lost');
    if (processing)
      return ChatRequestReceipt(
        id: request.id,
        status: "processing",
        expiresAt: DateTime.now().add(const Duration(seconds: 28)),
      );
    return super.submitRequest(request);
  }

  @override
  Future<ChatRequestReceipt> requestResult(String id) async {
    reads++;
    if (lostAcceptance && submitted.length == 1)
      throw const ApiException('not accepted', statusCode: 404);
    if (delayed) await hold.future;
    if (processing)
      return ChatRequestReceipt(
        id: id,
        status: "processing",
        expiresAt: DateTime.now().add(const Duration(seconds: 28)),
      );
    return ChatRequestReceipt(
      id: id,
      status: 'failed',
      expiresAt: DateTime.now(),
      errorCode: 'deadline_exceeded',
    );
  }

  @override
  Future<ChatRequestReceipt> cancelRequest(String id) async {
    cancellations++;
    if (missingCancellation)
      throw const ApiException('missing', statusCode: 404);
    return ChatRequestReceipt(
      id: id,
      status: 'cancelled',
      expiresAt: DateTime.now(),
    );
  }
}

void main() {
  test(
    'lost acceptance resubmits exact logical payload once, not a new chat',
    () async {
      final repository = RequestRepository()..lostAcceptance = true;
      final storage = MemoryPendingRequestStorage();
      final container = ProviderContainer(
        overrides: [
          consultationRepositoryProvider.overrideWithValue(repository),
          pendingRequestStorageProvider.overrideWithValue(storage),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(consultationControllerProvider.notifier);
      await vm.submit('通信断の相談', null);
      expect(repository.submitted.length, 2);
      expect(repository.submitted[1].id, repository.submitted[0].id);
      expect(repository.submitted[1].message, repository.submitted[0].message);
      expect(container.read(consultationControllerProvider).unresolved, false);
      expect(storage.value, isNull);
    },
  );
  test(
    'definitive rejection releases pending but busy keeps logical ID',
    () async {
      for (final code in [400, 409]) {
        final repository = RequestRepository()..rejectCode = code;
        final storage = MemoryPendingRequestStorage();
        final container = ProviderContainer(
          overrides: [
            consultationRepositoryProvider.overrideWithValue(repository),
            pendingRequestStorageProvider.overrideWithValue(storage),
          ],
        );
        final vm = container.read(consultationControllerProvider.notifier);
        await vm.submit('編集する元入力', null);
        expect(
          container.read(consultationControllerProvider).unresolved,
          code == 409,
        );
        expect(storage.value != null, code == 409);
        expect(repository.reads, 0);
        container.dispose();
      }
    },
  );
  test(
    'cancelled unaccepted restored request never submits even on 404',
    () async {
      final repository = RequestRepository()..missingCancellation = true;
      final storage = MemoryPendingRequestStorage()
        ..value = const PendingChatRequest(
          id: 'retired-request-001',
          message: '入力を保持',
          cancelRequested: true,
        );
      final container = ProviderContainer(
        overrides: [
          consultationRepositoryProvider.overrideWithValue(repository),
          pendingRequestStorageProvider.overrideWithValue(storage),
        ],
      );
      addTearDown(container.dispose);
      final vm = container.read(consultationControllerProvider.notifier);
      await Future<void>.delayed(Duration.zero);
      await vm.recover();
      expect(repository.submitted, isEmpty);
      expect(repository.cancellations, 1);
      expect(storage.value!.cancelRequested, true);
      expect(container.read(consultationControllerProvider).unresolved, true);
    },
  );
  testWidgets(
    'existing pending survives screen recreation and result button preserves failed input',
    (tester) async {
      final repository = RequestRepository();
      final storage = MemoryPendingRequestStorage()
        ..value = const PendingChatRequest(
          id: 'restored-request-001',
          message: '復元する相談',
        );
      final container = ProviderContainer(
        overrides: [
          consultationRepositoryProvider.overrideWithValue(repository),
          pendingRequestStorageProvider.overrideWithValue(storage),
        ],
      );
      addTearDown(container.dispose);
      container.read(consultationControllerProvider.notifier);
      await tester.pump();
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(home: Scaffold(body: ConsultationScreen())),
        ),
      );
      await tester.pumpAndSettle();
      final field = tester.widget<TextField>(
        find.byKey(const ValueKey('consultation-message')),
      );
      expect(field.controller!.text, '復元する相談');
      expect(field.enabled, false);
      await tester.tap(find.byKey(const ValueKey('submit-consultation')));
      await tester.pumpAndSettle();
      expect(repository.reads, 1);
      expect(repository.submitted, isEmpty);
      expect(
        tester
            .widget<TextField>(
              find.byKey(const ValueKey('consultation-message')),
            )
            .controller!
            .text,
        '復元する相談',
      );
      expect(container.read(consultationControllerProvider).unresolved, false);
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets('polling is bounded and retains original input and ID', (
    tester,
  ) async {
    final repository = RequestRepository()..processing = true;
    final storage = MemoryPendingRequestStorage();
    final container = ProviderContainer(
      overrides: [
        consultationRepositoryProvider.overrideWithValue(repository),
        pendingRequestStorageProvider.overrideWithValue(storage),
      ],
    );
    addTearDown(container.dispose);
    final vm = container.read(consultationControllerProvider.notifier);
    final work = vm.submit('最大待機', null);
    for (var i = 0; i < 50; i++) {
      await tester.pump(const Duration(seconds: 1));
    }
    await work;
    expect(repository.reads, lessThanOrEqualTo(24));
    expect(repository.submitted.length, 1);
    expect(storage.value!.id, repository.submitted.single.id);
    expect(container.read(consultationControllerProvider).unresolved, true);
    expect(
      container.read(consultationControllerProvider).consultation.hasError,
      true,
    );
  });
  test('session generation change discards late results', () async {
    final repository = RequestRepository()..delayed = true;
    final storage = MemoryPendingRequestStorage()
      ..value = const PendingChatRequest(
        id: 'old-session-request',
        message: '旧セッション',
      );
    final container = ProviderContainer(
      overrides: [
        consultationRepositoryProvider.overrideWithValue(repository),
        pendingRequestStorageProvider.overrideWithValue(storage),
      ],
    );
    addTearDown(container.dispose);
    final vm = container.read(consultationControllerProvider.notifier);
    await Future<void>.delayed(Duration.zero);
    final work = vm.recover();
    await Future<void>.delayed(Duration.zero);
    container.read(sessionControllerProvider).generation++;
    repository.hold.complete();
    await work;
    expect(container.read(consultationControllerProvider).unresolved, true);
    expect(storage.value, isNotNull);
  });
  test(
    'disposed provider ignores late receipt and retains pending for recovery',
    () async {
      final repository = RequestRepository()..delayed = true;
      final storage = MemoryPendingRequestStorage()
        ..value = const PendingChatRequest(
          id: 'late-request-001',
          message: '遅い結果',
        );
      final container = ProviderContainer(
        overrides: [
          consultationRepositoryProvider.overrideWithValue(repository),
          pendingRequestStorageProvider.overrideWithValue(storage),
        ],
      );
      final vm = container.read(consultationControllerProvider.notifier);
      await Future<void>.delayed(Duration.zero);
      final work = vm.recover();
      await Future<void>.delayed(Duration.zero);
      container.dispose();
      repository.hold.complete();
      await work;
      expect(storage.value, isNotNull);
    },
  );
}
