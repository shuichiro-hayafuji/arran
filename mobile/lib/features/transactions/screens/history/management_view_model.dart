import 'dart:math';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../shared/presentation/mvi.dart';
import '../../../../core/network/api_exception.dart';
import '../../domain/import_management.dart';
import '../../repository/import_management_repository.dart';
import '../../repository/reconciliation_repository.dart';
import '../list/transactions_view_model.dart';
import '../list/transactions_intent.dart';
import '../../../dashboard/screens/dashboard/dashboard_view_model.dart';
import '../../../dashboard/screens/dashboard/dashboard_intent.dart';
import 'management_intent.dart';
import 'management_state.dart';

String mutationId() => List.generate(
  16,
  (_) => Random.secure().nextInt(256).toRadixString(16).padLeft(2, '0'),
).join();

class ManagementViewModel
    extends MviViewModel<ManagementState, ManagementIntent> {
  ManagementViewModel(this.repository, this.target, this.isImport, this.ref)
    : super(const ManagementState());
  final ImportManagementRepository repository;
  final String target;
  final bool isImport;
  final Ref ref;
  @override
  Future<void> dispatch(ManagementIntent intent) async {
    if (!mounted || state.loading) return;
    if (intent is RecoverManagement && state.pending == null) return;
    if (intent is LoadManagement && state.pending != null) return;
    final request = intent is SaveManagement ? intent.request : state.pending;
    if (intent is SaveManagement && state.pending != null) return;
    final prior = state;
    state = ManagementState(
      batch: prior.batch,
      record: prior.record,
      audit: prior.audit,
      loading: true,
      pending: request,
      result: prior.result,
    );
    MutationResult? result = prior.result;
    try {
      if (intent is SaveManagement || intent is RecoverManagement) {
        if (request == null) return;
        if (intent is RecoverManagement) {
          try {
            result = await repository.result(request.id);
          } on ApiException catch (e) {
            if (e.statusCode != 404) rethrow;
            result = await repository.mutate(request);
          }
        } else {
          result = await repository.mutate(request);
        }
        if (!mounted) return;
        state = ManagementState(
          batch: prior.batch,
          record: prior.record,
          audit: prior.audit,
          loading: true,
          result: result,
        );
        ref.invalidate(expenseCountProvider);
        ref
            .read(transactionsViewModelProvider.notifier)
            .dispatch(const RefreshTransactions());
        ref
            .read(dashboardViewModelProvider.notifier)
            .dispatch(RefreshDashboard());
      }
      final batch = isImport ? await repository.batch(target) : null;
      final record = isImport
          ? null
          : await repository.transaction(int.parse(target));
      final audit = await repository.audit(target);
      if (!mounted) return;
      state = ManagementState(
        batch: batch,
        record: record,
        audit: audit,
        result: result,
      );
    } catch (error) {
      if (!mounted) return;
      final rejected =
          error is ApiException && [400, 404, 409].contains(error.statusCode);
      state = ManagementState(
        batch: state.batch,
        record: state.record,
        audit: state.audit,
        pending: rejected ? null : state.pending,
        result: state.result,
        error: error.toString(),
      );
    }
  }
}

final managementControllerProvider = StateNotifierProvider.autoDispose
    .family<ManagementViewModel, ManagementState, ({String id, bool isImport})>(
      (ref, key) {
        final vm = ManagementViewModel(
          ref.watch(importManagementRepositoryProvider),
          key.id,
          key.isImport,
          ref,
        );
        Future.microtask(() => vm.dispatch(const LoadManagement()));
        return vm;
      },
    );
final importHistoryProvider = FutureProvider.autoDispose
    .family<List<ImportBatch>, String>(
      (ref, before) =>
          ref.watch(importManagementRepositoryProvider).history(before),
    );
