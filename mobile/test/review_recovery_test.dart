import 'package:flutter/material.dart';
import 'dart:async';
import 'package:spendable_today/features/consultation/domain/consultation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/core/network/api_exception.dart';
import 'package:spendable_today/features/consultation/domain/chat_request.dart';
import 'package:spendable_today/features/consultation/repository/consultation_repository.dart';
import 'package:spendable_today/features/consultation/repository/pending_request_storage.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_screen.dart';
import 'package:spendable_today/features/consultation/screens/consultation/consultation_view_model.dart';
import 'widget_test.dart' show FakeConsultationRepository;

class ReviewRepository extends FakeConsultationRepository {
  bool reject = false;
  int rejectCode = 400;
  int submissions = 0;
  @override
  Future<ChatRequestReceipt> requestResult(String id) async {
    if (reject) throw const ApiException('not accepted', statusCode: 404);
    return super.submitRequest(PendingChatRequest(id: id, message: '元の相談'));
  }

  @override
  Future<ChatRequestReceipt> submitRequest(PendingChatRequest p) async {
    submissions++;
    if (reject) throw ApiException('bad input', statusCode: rejectCode);
    return super.submitRequest(p);
  }

  @override
  Future<ChatRequestReceipt> cancelRequest(String id) async =>
      ChatRequestReceipt(
        id: id,
        status: 'cancelled',
        expiresAt: DateTime.now(),
        result: const Consultation(
          id: 1,
          createdAt: '2026-10-12T00:00:00Z',
          userMessage: '元の相談',
          plannedAmount: null,
          inferredCategory: '',
          recommendation: '相談を中止し内容を保存しました',
          reasoningSummary: '',
          currentSituation: '',
          alternative: '',
          finalQuestion: '',
          status: 'unavailable',
          actualAmount: null,
          userDecisionReason: '',
          satisfactionScore: null,
          regretScore: null,
          note: '',
          responseSource: 'unavailable',
          needsFollowUp: false,
          followUpQuestion: '',
        ),
      );
}

class SlowHistoryRepository extends ReviewRepository {
  final release = Completer<void>();
  int histories = 0;
  @override
  Future<List<Consultation>> getAll() async {
    if (++histories > 1) await release.future;
    return super.getAll();
  }
}

void main() {
  for (final recovery in [false, true]) {
    testWidgets('new draft survives delayed history (recovery=$recovery)', (
      tester,
    ) async {
      final r = SlowHistoryRepository();
      final st = MemoryPendingRequestStorage();
      if (recovery)
        st.value = const PendingChatRequest(
          id: 'review-history-003',
          message: '元の相談',
        );
      final c = ProviderContainer(
        overrides: [
          consultationRepositoryProvider.overrideWithValue(r),
          pendingRequestStorageProvider.overrideWithValue(st),
        ],
      );
      addTearDown(c.dispose);
      c.read(consultationControllerProvider.notifier);
      await tester.pump();
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: c,
          child: const MaterialApp(home: Scaffold(body: ConsultationScreen())),
        ),
      );
      await tester.pumpAndSettle();
      if (!recovery)
        await tester.enterText(
          find.byKey(const ValueKey('consultation-message')),
          '元の相談',
        );
      await tester.tap(find.byKey(const ValueKey('submit-consultation')));
      await tester.pump();
      await tester.pump();
      await tester.enterText(
        find.byKey(const ValueKey('consultation-message')),
        '次の新しい相談',
      );
      r.release.complete();
      await tester.pumpAndSettle();
      final field = tester.widget<TextField>(
        find.byKey(const ValueKey('consultation-message')),
      );
      expect(field.controller!.text, '次の新しい相談');
    });
  }

  for (final code in [400, 404]) {
    test('restored rejected $code payload becomes editable', () async {
      final r = ReviewRepository()
        ..reject = true
        ..rejectCode = code;
      final st = MemoryPendingRequestStorage()
        ..value = const PendingChatRequest(
          id: 'review-recovery-001',
          message: '元の相談',
        );
      final c = ProviderContainer(
        overrides: [
          consultationRepositoryProvider.overrideWithValue(r),
          pendingRequestStorageProvider.overrideWithValue(st),
        ],
      );
      addTearDown(c.dispose);
      final vm = c.read(consultationControllerProvider.notifier);
      await Future<void>.delayed(Duration.zero);
      await vm.recover();
      expect(c.read(consultationControllerProvider).unresolved, false);
      expect(st.value, isNull);
    });
  }
  for (final cancelled in [false, true]) {
    testWidgets('saved restored receipt clears input (cancelled=$cancelled)', (
      tester,
    ) async {
      final r = ReviewRepository();
      final st = MemoryPendingRequestStorage()
        ..value = PendingChatRequest(
          id: 'review-recovery-002',
          message: '元の相談',
          cancelRequested: cancelled,
        );
      final c = ProviderContainer(
        overrides: [
          consultationRepositoryProvider.overrideWithValue(r),
          pendingRequestStorageProvider.overrideWithValue(st),
        ],
      );
      addTearDown(c.dispose);
      c.read(consultationControllerProvider.notifier);
      await tester.pump();
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: c,
          child: const MaterialApp(home: Scaffold(body: ConsultationScreen())),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('submit-consultation')));
      await tester.pumpAndSettle();
      expect(c.read(consultationControllerProvider).unresolved, false);
      final field = tester.widget<TextField>(
        find.byKey(const ValueKey('consultation-message')),
      );
      expect(field.controller!.text, isEmpty);
      await tester.tap(find.byKey(const ValueKey('submit-consultation')));
      await tester.pumpAndSettle();
      expect(r.submissions, 0);
    });
  }
}
