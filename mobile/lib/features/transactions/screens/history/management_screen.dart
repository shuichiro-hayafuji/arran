import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../shared/utils/formatters.dart';
import '../../domain/categories.dart';
import '../../domain/import_management.dart';
import 'management_view_model.dart';
import 'management_intent.dart';

class ManagementScreen extends ConsumerStatefulWidget {
  const ManagementScreen({required this.id, required this.isImport, super.key});
  final String id;
  final bool isImport;
  @override
  ConsumerState<ManagementScreen> createState() => _ManagementScreenState();
}

class _ManagementScreenState extends ConsumerState<ManagementScreen> {
  final amount = TextEditingController(), date = TextEditingController();
  String? category;
  bool remember = false;
  int? revision;
  String? inputError;
  @override
  void dispose() {
    amount.dispose();
    date.dispose();
    super.dispose();
  }

  Future<void> confirm(MutationRequest request, String text) async {
    final agreed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('変更を確認'),
        content: Text(text),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('戻る'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('変更を保存'),
          ),
        ],
      ),
    );
    if (agreed == true && mounted)
      await ref
          .read(
            managementControllerProvider((
              id: widget.id,
              isImport: widget.isImport,
            )).notifier,
          )
          .dispatch(SaveManagement(request));
  }

  @override
  Widget build(BuildContext context) {
    final key = (id: widget.id, isImport: widget.isImport);
    final state = ref.watch(managementControllerProvider(key));
    final record = state.record;
    if (record != null && revision != record.revision) {
      revision = record.revision;
      amount.text = record.current.amount.toString();
      date.text = record.current.transactionDate;
      category = record.current.category;
    }
    final disabled = state.loading || state.pending != null;
    return Scaffold(
      appBar: AppBar(title: Text(widget.isImport ? '取込詳細' : '取引詳細')),
      body: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          if (state.loading) const LinearProgressIndicator(),
          if (state.error != null) Text(state.error!),
          if (inputError != null) Text(inputError!),
          if (state.pending != null) ...[
            const Text('保存結果を確認できていません。同じ操作の結果を確認してください。'),
            FilledButton(
              onPressed: state.loading
                  ? null
                  : () => ref
                        .read(managementControllerProvider(key).notifier)
                        .dispatch(const RecoverManagement()),
              child: const Text('保存結果を確認'),
            ),
          ],
          if (state.pending == null)
            TextButton(
              onPressed: state.loading
                  ? null
                  : () => ref
                        .read(managementControllerProvider(key).notifier)
                        .dispatch(const LoadManagement()),
              child: const Text('最新状態を読み直す'),
            ),
          if (state.result != null) ...[
            Text(
              '変更 ${state.result!.affected}件、他の取込で保護 ${state.result!.protected}件',
            ),
            const Text('銀行記録の変更を保存しました。残高・自由予算への反映は次の相談で確認します。'),
            TextButton(
              onPressed: () => context.go('/consult'),
              child: const Text('相談で金融反映を確認'),
            ),
          ],
          if (state.batch case final batch?) ...[
            Text(
              batch.fileName.isEmpty ? 'ファイル名不明' : batch.fileName,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            Text(batch.recordedAt.isEmpty ? '取込日時不明' : batch.recordedAt),
            Text('${batch.source} / ${batch.account}'),
            const Text('銀行原記録・訂正履歴・本人の購入実績を保持します。元のCSVファイルは保存していません。'),
            if (batch.status == 'active')
              Text(
                '取消で非表示になる明細 ${batch.affected}件、他の有効な証拠で残る明細 ${batch.protected}件',
              ),
            FilledButton(
              onPressed: disabled
                  ? null
                  : () => confirm(
                      MutationRequest(
                        id: mutationId(),
                        target: batch.id,
                        action: batch.status == 'active' ? 'cancel' : 'restore',
                        stateVersion: batch.stateVersion,
                        version: batch.version,
                        snapshot: batch.snapshot,
                      ),
                      batch.status == 'active'
                          ? 'この取込を取り消します。他の有効な取込や本人の購入は取り消しません。残高は自動返金しません。'
                          : 'この取込を再び有効にします。同じ明細IDと最新訂正を保持し、残高は自動変更しません。',
                    ),
              child: Text(
                batch.status == 'active' ? 'この取込を取り消す' : 'この取込を再び有効にする',
              ),
            ),
            for (final row in batch.rows)
              ListTile(
                title: Text(row.current.description),
                subtitle: Text(
                  '${row.current.transactionDate} ${yen(row.current.amount)} ${row.active ? '有効' : '非表示'}',
                ),
                onTap: () => context.push('/transactions/${row.transactionId}'),
              ),
          ],
          if (record != null) ...[
            Text(
              record.current.description,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            Text(record.active ? '有効な銀行明細' : '取込取消により非表示の銀行明細'),
            Text(
              '銀行原記録: ${record.original.transactionDate} ${yen(record.original.amount)} ${record.original.category}',
            ),
            TextField(
              controller: amount,
              enabled: !disabled,
              keyboardType: TextInputType.number,
              decoration: const InputDecoration(labelText: '訂正後の金額（円）'),
            ),
            TextField(
              controller: date,
              enabled: !disabled,
              decoration: const InputDecoration(
                labelText: '訂正後の日付（YYYY-MM-DD）',
              ),
            ),
            DropdownButtonFormField<String>(
              key: ValueKey('${record.revision}:$category'),
              initialValue: categories.contains(category) ? category : null,
              decoration: const InputDecoration(labelText: 'カテゴリ'),
              items: categories
                  .map((c) => DropdownMenuItem(value: c, child: Text(c)))
                  .toList(),
              onChanged: disabled ? null : (v) => setState(() => category = v),
            ),
            CheckboxListTile(
              value: remember,
              onChanged: disabled
                  ? null
                  : (v) => setState(() => remember = v ?? false),
              title: const Text('同じ加盟店に次回もこのカテゴリを適用'),
            ),
            FilledButton(
              onPressed: disabled
                  ? null
                  : () {
                      final value = int.tryParse(amount.text.trim());
                      final parsed = DateTime.tryParse(date.text.trim());
                      if (value == null ||
                          value < 0 ||
                          value > 1000000000000 ||
                          parsed == null ||
                          parsed.toIso8601String().substring(0, 10) !=
                              date.text.trim() ||
                          category == null) {
                        setState(() => inputError = '金額・日付・カテゴリを確認してください。');
                        return;
                      }
                      setState(() => inputError = null);
                      confirm(
                        MutationRequest(
                          id: mutationId(),
                          target: widget.id,
                          action: 'correct',
                          stateVersion: record.stateVersion,
                          version: record.revision,
                          amount: value,
                          date: date.text.trim(),
                          category: category,
                          remember: remember,
                        ),
                        '銀行原記録を残して、金額・日付・カテゴリの訂正を保存します。本人の購入実績は別に保持し、金融反映は相談で確認します。',
                      );
                    },
              child: const Text('訂正を保存'),
            ),
          ],
          if (state.audit.isNotEmpty) ...[
            const Divider(),
            const Text('変更履歴（元の記録を保持）'),
            for (final event in state.audit)
              ExpansionTile(
                title: Text('${auditLabel(event.kind)} ${event.recordedAt}'),
                children: [
                  Text(
                    '変更前 ${event.before.length}件 / 変更後 ${event.after.length}件',
                  ),
                  for (final row in event.before.take(20))
                    Text(
                      '前: ${row.transactionDate} ${yen(row.amount)} ${row.category}',
                    ),
                  for (final row in event.after.take(20))
                    Text(
                      '後: ${row.transactionDate} ${yen(row.amount)} ${row.category}',
                    ),
                ],
              ),
          ],
        ],
      ),
    );
  }
}

String auditLabel(String kind) => switch (kind) {
  'correct' => '訂正',
  'cancel' => '取込取消',
  'restore' => '再有効化',
  'financial_confirmation' => '金融反映の確認',
  _ => '変更',
};
