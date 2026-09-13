import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:spendable_today/features/consultation/domain/consultation.dart';
import 'package:spendable_today/features/consultation/screens/result/result_intent.dart';
import 'package:spendable_today/features/consultation/screens/result/result_view_model.dart';

class ResultScreen extends ConsumerStatefulWidget {
  const ResultScreen({required this.consultationId, super.key});

  final int consultationId;

  @override
  ConsumerState<ResultScreen> createState() => _ResultScreenState();
}

class _ResultScreenState extends ConsumerState<ResultScreen> {
  var _status = 'skipped';
  final _actual = TextEditingController();
  final _reason = TextEditingController();
  final _note = TextEditingController();
  var _satisfaction = 3.0;
  var _regret = 3.0;
  var _hydrated = false;

  @override
  void dispose() {
    _actual.dispose();
    _reason.dispose();
    _note.dispose();
    super.dispose();
  }

  Future<void> _save() => ref
      .read(resultViewModelProvider(widget.consultationId).notifier)
      .dispatch(
        SaveResult(
          status: _status,
          actualAmount: int.tryParse(_actual.text),
          reason: _reason.text.trim(),
          satisfaction: _satisfaction.round(),
          regret: _regret.round(),
          note: _note.text.trim(),
        ),
      );

  void _hydrate(Consultation consultation) {
    if (_hydrated) return;
    _hydrated = true;
    if (const [
      'spent',
      'skipped',
      'reduced',
      'pending',
    ].contains(consultation.status)) {
      _status = consultation.status;
    }
    _actual.text = consultation.actualAmount?.toString() ?? '';
    _reason.text = consultation.userDecisionReason;
    _note.text = consultation.note;
    _satisfaction = (consultation.satisfactionScore ?? 3).toDouble();
    _regret = (consultation.regretScore ?? 3).toDouble();
  }

  @override
  Widget build(BuildContext context) {
    final resultState = ref.watch(
      resultViewModelProvider(widget.consultationId),
    );
    ref.listen(resultViewModelProvider(widget.consultationId), (
      previous,
      next,
    ) {
      if (next.saved && previous?.saved != true && context.mounted) {
        final navigator = Navigator.of(context);
        if (navigator.canPop()) {
          navigator.pop();
        }
      }
      if (next.actionError != null &&
          next.actionError != previous?.actionError &&
          context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(next.actionError.toString())));
      }
    });
    return Scaffold(
      appBar: AppBar(title: const Text('相談結果を記録')),
      body: resultState.consultation.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(child: Text(error.toString())),
        data: (items) {
          final consultation = items;
          if (consultation != null) _hydrate(consultation);
          return ListView(
            padding: const EdgeInsets.fromLTRB(20, 8, 20, 36),
            children: [
              if (consultation != null)
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(18),
                    child: Text(
                      consultation.userMessage,
                      style: const TextStyle(fontWeight: FontWeight.w700),
                    ),
                  ),
                ),
              const SizedBox(height: 16),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(value: 'spent', label: Text('支出した')),
                  ButtonSegment(value: 'skipped', label: Text('見送った')),
                  ButtonSegment(value: 'reduced', label: Text('減らした')),
                  ButtonSegment(value: 'pending', label: Text('保留')),
                ],
                selected: {_status},
                onSelectionChanged: (values) {
                  setState(() => _status = values.first);
                },
                showSelectedIcon: false,
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _actual,
                keyboardType: TextInputType.number,
                inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                decoration: const InputDecoration(
                  labelText: '実際の金額（任意）',
                  suffixText: '円',
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _reason,
                minLines: 2,
                maxLines: 4,
                decoration: const InputDecoration(
                  labelText: '決定理由（メモリの根拠になります）',
                ),
              ),
              const SizedBox(height: 18),
              Text('満足度: ${_satisfaction.round()} / 5'),
              Slider(
                value: _satisfaction,
                min: 1,
                max: 5,
                divisions: 4,
                onChanged: (value) => setState(() => _satisfaction = value),
              ),
              Text('後悔度: ${_regret.round()} / 5'),
              Slider(
                value: _regret,
                min: 1,
                max: 5,
                divisions: 4,
                onChanged: (value) => setState(() => _regret = value),
              ),
              TextField(
                controller: _note,
                minLines: 2,
                maxLines: 4,
                decoration: const InputDecoration(labelText: '自由記述（任意）'),
              ),
              const SizedBox(height: 20),
              FilledButton(
                key: const ValueKey('save-consultation-result'),
                onPressed: resultState.saving ? null : _save,
                child: Padding(
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  child: Text(resultState.saving ? '保存中…' : '結果を保存'),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}
