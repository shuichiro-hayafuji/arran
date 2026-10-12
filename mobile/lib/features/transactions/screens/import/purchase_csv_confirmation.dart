import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/constants/space_theme.dart';
import '../../domain/import_preview.dart';
import 'purchase_csv_controller.dart';

class PurchaseCsvConfirmation extends ConsumerStatefulWidget {
  const PurchaseCsvConfirmation({
    super.key,
    required this.purchaseId,
    required this.description,
  });
  final String purchaseId;
  final String description;
  @override
  ConsumerState<PurchaseCsvConfirmation> createState() =>
      _PurchaseCsvConfirmationState();
}

class _PurchaseCsvConfirmationState
    extends ConsumerState<PurchaseCsvConfirmation> {
  int? selected;
  String? selectedRevision;
  @override
  Widget build(BuildContext context) {
    final state = ref.watch(purchaseCsvControllerProvider(widget.purchaseId));
    final controller = ref.read(
      purchaseCsvControllerProvider(widget.purchaseId).notifier,
    );
    final revision = state.candidates
        .map(
          (c) =>
              '${c.row}/${c.transaction.id}/${c.stateVersion}/${c.candidates.map((p) => '${p.id}/${p.snapshot}').join(',')}',
        )
        .join(';');
    final selectionCurrent = selectedRevision == revision;
    return Card(
      child: Padding(
        padding: AppSpace.pM,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text('${widget.description}：CSVとの照合・金融反映待ち'),
            if (state.loading) const LinearProgressIndicator(),
            if (state.error != null) ...[
              Text(state.error!),
              TextButton(
                onPressed: () => controller.dispatch(const LoadPurchaseCsv()),
                child: const Text('照合候補を再取得'),
              ),
            ],
            if (!state.loading &&
                state.error == null &&
                state.candidates.isEmpty)
              const Text('照合済みの場合、残高・自由予算の質問にはチャットで回答してください。'),
            if (state.candidates.isNotEmpty) ...[
              DropdownButtonFormField<int>(
                key: ValueKey(revision),
                isExpanded: true,
                decoration: const InputDecoration(labelText: '同じCSV明細を選ぶ'),
                items: [
                  const DropdownMenuItem(
                    value: 0,
                    child: Text('どのCSV明細とも別の購入'),
                  ),
                  for (final c in state.candidates)
                    DropdownMenuItem(
                      value: c.transaction.id,
                      child: Text(
                        '${c.transaction.transactionDate} ${c.transaction.description} ${c.transaction.amount}円',
                        maxLines: 2,
                      ),
                    ),
                ],
                onChanged: state.loading
                    ? null
                    : (value) => setState(() {
                        selected = value;
                        selectedRevision = revision;
                      }),
              ),
              FilledButton(
                onPressed:
                    state.loading ||
                        selected == null ||
                        !selectionCurrent ||
                        (selected != 0 &&
                            !state.candidates.any(
                              (c) => c.transaction.id == selected,
                            ))
                    ? null
                    : () {
                        final c = selected == 0
                            ? state.candidates.first
                            : state.candidates.firstWhere(
                                (c) => c.transaction.id == selected,
                              );
                        final p = c.candidates.firstWhere(
                          (p) => p.id == widget.purchaseId,
                        );
                        controller.dispatch(
                          ConfirmPurchaseCsv(
                            selected!,
                            PurchaseCsvDecision(
                              row: c.row,
                              action: selected == 0 ? 'distinct' : 'same',
                              purchaseId: p.id,
                              snapshot: p.snapshot,
                              stateVersion: c.stateVersion,
                            ),
                          ),
                        );
                      },
                child: const Text('同一性を確認して保存'),
              ),
            ],
            if (state.saved) const Text('照合を保存しました。金融反映は次のチャットで確定します。'),
          ],
        ),
      ),
    );
  }
}
