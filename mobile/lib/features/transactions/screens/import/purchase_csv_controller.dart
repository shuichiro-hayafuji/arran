import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../shared/presentation/mvi.dart';
import '../../domain/import_preview.dart';
import '../../repository/reconciliation_repository.dart';

sealed class PurchaseCsvIntent implements MviIntent {
  const PurchaseCsvIntent();
}

class LoadPurchaseCsv extends PurchaseCsvIntent {
  const LoadPurchaseCsv();
}

class ConfirmPurchaseCsv extends PurchaseCsvIntent {
  const ConfirmPurchaseCsv(this.transactionId, this.decision);
  final int transactionId;
  final PurchaseCsvDecision decision;
}

class PurchaseCsvState {
  const PurchaseCsvState({
    this.candidates = const [],
    this.loading = false,
    this.saved = false,
    this.error,
  });
  final List<PurchaseCsvConflict> candidates;
  final bool loading;
  final bool saved;
  final String? error;
}

class PurchaseCsvViewModel
    extends MviViewModel<PurchaseCsvState, PurchaseCsvIntent> {
  PurchaseCsvViewModel(this.repository, this.purchaseId, this.ref)
    : super(const PurchaseCsvState());
  final ReconciliationRepository repository;
  final String purchaseId;
  final Ref ref;
  int generation = 0;
  @override
  Future<void> dispatch(PurchaseCsvIntent intent) async {
    if (!mounted || state.loading) return;
    final current = ++generation;
    final previous = state.candidates;
    state = PurchaseCsvState(candidates: previous, loading: true);
    try {
      if (intent is ConfirmPurchaseCsv) {
        await repository.confirm(intent.transactionId, intent.decision);
        if (!mounted || current != generation) return;
        ref.invalidate(expenseCountProvider);
      }
      final values = await repository.candidates(purchaseId);
      if (!mounted || current != generation) return;
      state = PurchaseCsvState(
        candidates: values,
        saved: intent is ConfirmPurchaseCsv,
      );
    } catch (error) {
      if (!mounted || current != generation) return;
      state = PurchaseCsvState(candidates: previous, error: error.toString());
    }
  }
}

final purchaseCsvControllerProvider = StateNotifierProvider.autoDispose
    .family<PurchaseCsvViewModel, PurchaseCsvState, String>((ref, id) {
      final controller = PurchaseCsvViewModel(
        ref.watch(reconciliationRepositoryProvider),
        id,
        ref,
      );
      Future.microtask(() => controller.dispatch(const LoadPurchaseCsv()));
      return controller;
    });
