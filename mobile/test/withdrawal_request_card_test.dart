import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/features/profile/repository/withdrawal_request_repository.dart';
import 'package:spendable_today/features/profile/screens/settings/withdrawal_request_card.dart';

class FakeWithdrawalRepository implements WithdrawalRequestRepository {
  bool requested = false;
  bool lostResponse = false;
  bool failStatusAfterSubmit = false;
  int? sentGeneration;
  @override
  Future<bool> get() async {
    if (requested && failStatusAfterSubmit)
      throw StateError('status unavailable');
    return requested;
  }

  @override
  Future<void> submit(int generation) async {
    sentGeneration = generation;
    requested = true;
    if (lostResponse) throw StateError('response lost');
  }
}

void main() {
  testWidgets('confirmation and lost response preserve the recorded state', (
    tester,
  ) async {
    final repository = FakeWithdrawalRepository()..lostResponse = true;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          withdrawalRequestRepositoryProvider.overrideWithValue(repository),
        ],
        child: const MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(child: WithdrawalRequestCard()),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('退会を申し込む'));
    await tester.pumpAndSettle();
    expect(repository.requested, false);
    await tester.tap(find.text('申し込む'));
    await tester.pumpAndSettle();
    expect(repository.requested, true);
    expect(repository.sentGeneration, isNotNull);
    expect(find.text('申し込みを記録しました。運営者による確認・削除は未完了です。'), findsOneWidget);
    expect(find.text('申し込み結果を確認できません。状態を再確認してください。'), findsNothing);
  });
  testWidgets('confirmed response and failed status do not contradict', (
    tester,
  ) async {
    final repository = FakeWithdrawalRepository()..failStatusAfterSubmit = true;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          withdrawalRequestRepositoryProvider.overrideWithValue(repository),
        ],
        child: const MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(child: WithdrawalRequestCard()),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('退会を申し込む'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('申し込む'));
    await tester.pumpAndSettle();
    expect(
      find.text('申し込みの応答を受信しましたが、現在の状態を確認できません。再読み込みしてください。'),
      findsOneWidget,
    );
    expect(find.text('申し込みを記録しました。運営者による確認・削除は未完了です。'), findsNothing);
  });
}
