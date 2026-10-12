import 'package:flutter/material.dart';
import '../../domain/import_preview.dart';
import '../../../../app/constants/space_theme.dart';

class PurchaseCsvForm extends StatefulWidget {
  const PurchaseCsvForm({
    super.key,
    required this.conflicts,
    required this.onSubmit,
    this.loading = false,
  });
  final List<PurchaseCsvConflict> conflicts;
  final ValueChanged<List<PurchaseCsvDecision>> onSubmit;
  final bool loading;
  @override
  State<PurchaseCsvForm> createState() => _PurchaseCsvFormState();
}

class _PurchaseCsvFormState extends State<PurchaseCsvForm> {
  final Map<int, String> choices = {};
  @override
  void didUpdateWidget(covariant PurchaseCsvForm oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.conflicts.length != widget.conflicts.length ||
        oldWidget.conflicts.any(
          (old) => !widget.conflicts.any(
            (c) => c.row == old.row && c.stateVersion == old.stateVersion,
          ),
        )) {
      choices.clear();
    }
  }

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      Text('チャットの購入と同じ支出ですか', style: Theme.of(context).textTheme.titleLarge),
      const Text('日付や金額が似ていても自動でまとめません。相談日時は実購入日ではありません。'),
      for (final conflict in widget.conflicts)
        Padding(
          padding: AppSpace.pM,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '${conflict.transaction.transactionDate} ${conflict.transaction.description} ${conflict.transaction.amount}円',
              ),
              DropdownButtonFormField<String>(
                key: ValueKey('${conflict.row}/${conflict.stateVersion}'),
                isExpanded: true,
                decoration: const InputDecoration(labelText: '同じ購入・別の支出を選ぶ'),
                items: [
                  const DropdownMenuItem(
                    value: 'distinct',
                    child: Text('どの購入とも別の支出'),
                  ),
                  for (final c in conflict.candidates)
                    DropdownMenuItem(
                      value: c.id,
                      child: Text(
                        '${c.description} ${c.amount}円・${c.status == 'not_purchased'
                            ? '未購入へ訂正済み'
                            : c.status == 'needs_classification'
                            ? '用途を確認中'
                            : '購入実績'}\n${c.purchaseDate.isEmpty ? '購入日不明／相談日時 ' : '購入日 '}${c.purchaseDate.isEmpty ? c.recordedAt : c.purchaseDate}',
                        maxLines: 3,
                      ),
                    ),
                ],
                onChanged: widget.loading
                    ? null
                    : (value) {
                        if (value != null)
                          setState(() => choices[conflict.row] = value);
                      },
              ),
            ],
          ),
        ),
      FilledButton(
        onPressed:
            widget.loading ||
                widget.conflicts.any((c) => !choices.containsKey(c.row))
            ? null
            : () {
                widget.onSubmit(
                  widget.conflicts.map((c) {
                    final value = choices[c.row]!;
                    if (value == 'distinct')
                      return PurchaseCsvDecision(
                        row: c.row,
                        action: 'distinct',
                        stateVersion: c.stateVersion,
                      );
                    final candidate = c.candidates.firstWhere(
                      (p) => p.id == value,
                    );
                    return PurchaseCsvDecision(
                      row: c.row,
                      action: 'same',
                      purchaseId: candidate.id,
                      snapshot: candidate.snapshot,
                      stateVersion: c.stateVersion,
                    );
                  }).toList(),
                );
              },
        child: const Text('購入との照合を確認'),
      ),
    ],
  );
}
