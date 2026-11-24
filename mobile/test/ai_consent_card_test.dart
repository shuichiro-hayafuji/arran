import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:spendable_today/core/network/api_exception.dart';
import 'package:spendable_today/features/profile/repository/ai_consent_repository.dart';
import 'package:spendable_today/features/profile/screens/settings/ai_consent_card.dart';

class FakeConsentRepository implements AiConsentRepository {
  bool failStatus = false;
  bool failWithdraw = false;
  bool failGrantAfterSave = false;
  bool rejectGrantForWithdrawal = false;
  bool failWithdrawAfterSave = false;
  bool unknownConflict = false;
  bool granted = false;
  int withdrawals = 0;
  String? grantedVersion;

  @override
  Future<AiConsentStatus> get() async {
    if (failStatus) throw StateError('unavailable');
    return AiConsentStatus(
      version: 'v1',
      description: '表示された説明',
      granted: granted,
      pending: false,
    );
  }

  @override
  Future<void> grant(String version, {required int generation}) async {
    if (unknownConflict)
      throw const ApiException('untrusted', statusCode: 409, code: 'unknown');
    if (rejectGrantForWithdrawal) {
      throw const ApiException(
        'ignored',
        statusCode: 409,
        code: 'withdrawal_pending',
      );
    }
    grantedVersion = version;
    granted = true;
    if (failGrantAfterSave) throw StateError('response lost');
  }

  @override
  Future<void> withdraw({required int generation}) async {
    withdrawals++;
    if (failWithdraw) throw StateError('unavailable');
    granted = false;
    if (failWithdrawAfterSave) throw StateError('response lost');
  }
}

void main() {
  Future<void> pumpCard(
    WidgetTester tester,
    FakeConsentRepository repository,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [aiConsentRepositoryProvider.overrideWithValue(repository)],
        child: const MaterialApp(
          home: Scaffold(body: SingleChildScrollView(child: AiConsentCard())),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('displays explanation and grants the displayed version', (
    tester,
  ) async {
    final repository = FakeConsentRepository();
    await pumpCard(tester, repository);
    expect(find.text('表示された説明'), findsOneWidget);
    await tester.tap(find.text('説明を確認して同意する'));
    await tester.pumpAndSettle();
    expect(repository.grantedVersion, 'v1');
    expect(find.text('この説明に同意しています。'), findsOneWidget);
  });

  testWidgets('reports a definite withdrawal conflict', (tester) async {
    final repository = FakeConsentRepository()..rejectGrantForWithdrawal = true;
    await pumpCard(tester, repository);
    await tester.tap(find.text('説明を確認して同意する'));
    await tester.pumpAndSettle();
    expect(find.text('退会申し込みの処理中は新しく同意できません。'), findsOneWidget);
    expect(find.text('操作結果を確認できません。再取得した状態が後から変わる可能性があります。'), findsNothing);
  });

  testWidgets('unknown conflict remains uncertain', (tester) async {
    final repository = FakeConsentRepository()..unknownConflict = true;
    await pumpCard(tester, repository);
    await tester.tap(find.text('説明を確認して同意する'));
    await tester.pumpAndSettle();
    expect(find.text('操作結果を確認できません。再取得した状態が後から変わる可能性があります。'), findsOneWidget);
    expect(find.text('untrusted'), findsNothing);
  });

  testWidgets('withdrawal remains available when policy cannot be loaded', (
    tester,
  ) async {
    final repository = FakeConsentRepository()..failStatus = true;
    await pumpCard(tester, repository);
    expect(find.text('同意の説明と状態を現在確認できません。'), findsOneWidget);
    await tester.tap(find.text('同意を撤回する'));
    await tester.pumpAndSettle();
    expect(repository.withdrawals, 1);
    expect(find.text('同意を撤回する'), findsOneWidget);
  });

  testWidgets('failed withdrawal does not report success', (tester) async {
    final repository = FakeConsentRepository()
      ..failStatus = true
      ..failWithdraw = true;
    await pumpCard(tester, repository);
    await tester.tap(find.text('同意を撤回する'));
    await tester.pumpAndSettle();
    expect(repository.withdrawals, 1);
    expect(find.text('操作結果を確認できません。再取得した状態が後から変わる可能性があります。'), findsOneWidget);
  });

  testWidgets('lost mutation response is uncertain and state is reloaded', (
    tester,
  ) async {
    final repository = FakeConsentRepository()..failGrantAfterSave = true;
    await pumpCard(tester, repository);
    await tester.tap(find.text('説明を確認して同意する'));
    await tester.pumpAndSettle();
    expect(find.text('この説明に同意しています。'), findsOneWidget);
    expect(find.text('操作結果を確認できません。再取得した状態が後から変わる可能性があります。'), findsOneWidget);
    repository.failWithdrawAfterSave = true;
    await tester.tap(find.text('同意を撤回する'));
    await tester.pumpAndSettle();
    expect(find.text('まだ同意していません。'), findsOneWidget);
    expect(find.text('操作結果を確認できません。再取得した状態が後から変わる可能性があります。'), findsOneWidget);
  });
}
