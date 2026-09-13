import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/screens/import/import_intent.dart';
import 'package:spendable_today/features/transactions/providers/import_controller_provider.dart';
import 'package:spendable_today/shared/utils/formatters.dart';

class ImportScreen extends ConsumerWidget {
  const ImportScreen({super.key});

  Future<void> _pick(WidgetRef ref) async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['csv'],
    );
    if (result == null || result.files.isEmpty) return;
    final file = result.files.first;
    if (file.path == null) return;
    await ref
        .read(importControllerProvider.notifier)
        .dispatch(SelectImportFile(filePath: file.path!, fileName: file.name));
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(importControllerProvider);
    final controller = ref.read(importControllerProvider.notifier);
    return Scaffold(
      appBar: AppBar(title: const Text('CSVインポート')),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 36),
        children: [
          Text(
            '利用履歴を追加',
            style: Theme.of(
              context,
            ).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w800),
          ),
          const SizedBox(height: 8),
          const Text('UTF-8、Shift_JIS/CP932に対応。確定するまでDBには保存されません。'),
          const SizedBox(height: 20),
          OutlinedButton.icon(
            key: const ValueKey('pick-csv'),
            onPressed: state.loading ? null : () => _pick(ref),
            icon: const Icon(Icons.file_open_outlined),
            label: Padding(
              padding: const EdgeInsets.symmetric(vertical: 13),
              child: Text(state.fileName ?? 'CSVファイルを選択'),
            ),
          ),
          if (state.loading) ...[
            const SizedBox(height: 16),
            const LinearProgressIndicator(),
          ],
          if (state.error != null) ...[
            const SizedBox(height: 14),
            Text(
              state.error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ],
          if (state.preview?.needsMapping == true) ...[
            const SizedBox(height: 20),
            _MappingForm(
              preview: state.preview!,
              onSubmit: (mapping) =>
                  controller.dispatch(ApplyImportMapping(mapping)),
            ),
          ] else if (state.preview != null) ...[
            const SizedBox(height: 20),
            _Preview(
              preview: state.preview!,
              onCommit: () => controller.dispatch(const CommitImport()),
              loading: state.loading,
            ),
          ],
          if (state.commitResult != null) ...[
            const SizedBox(height: 20),
            Card(
              color: Theme.of(context).colorScheme.primaryContainer,
              child: Padding(
                padding: const EdgeInsets.all(20),
                child: Column(
                  children: [
                    const Icon(Icons.check_circle_outline, size: 42),
                    const SizedBox(height: 8),
                    Text(
                      '${state.commitResult!.importedCount}件を保存しました',
                      style: Theme.of(context).textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    Text('重複 ${state.commitResult!.duplicateCount}件'),
                    const SizedBox(height: 12),
                    FilledButton(
                      onPressed: () {
                        controller.dispatch(const ResetImport());
                        context.go('/transactions');
                      },
                      child: const Text('取引一覧を見る'),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _MappingForm extends StatefulWidget {
  const _MappingForm({required this.preview, required this.onSubmit});

  final ImportPreview preview;
  final ValueChanged<CsvMapping> onSubmit;

  @override
  State<_MappingForm> createState() => _MappingFormState();
}

class _MappingFormState extends State<_MappingForm> {
  late String? _date = _valid(widget.preview.mapping.dateColumn);
  late String? _description = _valid(widget.preview.mapping.descriptionColumn);
  late String? _amount = _valid(widget.preview.mapping.amountColumn);
  late String? _debit = _valid(widget.preview.mapping.debitColumn);
  late String? _credit = _valid(widget.preview.mapping.creditColumn);

  String? _valid(String value) => value.isEmpty ? null : value;

  @override
  Widget build(BuildContext context) {
    final headers = widget.preview.headers;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(18),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              '列を対応付ける',
              style: TextStyle(fontWeight: FontWeight.w800, fontSize: 18),
            ),
            const SizedBox(height: 12),
            _ColumnDropdown(
              label: '日付列',
              value: _date,
              headers: headers,
              onChanged: (value) => setState(() => _date = value),
            ),
            _ColumnDropdown(
              label: '摘要・加盟店列',
              value: _description,
              headers: headers,
              onChanged: (value) => setState(() => _description = value),
            ),
            _ColumnDropdown(
              label: '金額列（または下の入出金列）',
              value: _amount,
              headers: headers,
              optional: true,
              onChanged: (value) => setState(() => _amount = value),
            ),
            _ColumnDropdown(
              label: '出金列',
              value: _debit,
              headers: headers,
              optional: true,
              onChanged: (value) => setState(() => _debit = value),
            ),
            _ColumnDropdown(
              label: '入金列',
              value: _credit,
              headers: headers,
              optional: true,
              onChanged: (value) => setState(() => _credit = value),
            ),
            const SizedBox(height: 8),
            FilledButton(
              key: const ValueKey('apply-mapping'),
              onPressed:
                  _date == null ||
                      _description == null ||
                      (_amount == null && _debit == null && _credit == null)
                  ? null
                  : () => widget.onSubmit(
                      CsvMapping(
                        dateColumn: _date!,
                        descriptionColumn: _description!,
                        amountColumn: _amount ?? '',
                        debitColumn: _debit ?? '',
                        creditColumn: _credit ?? '',
                      ),
                    ),
              child: const Text('この列でプレビュー'),
            ),
          ],
        ),
      ),
    );
  }
}

class _ColumnDropdown extends StatelessWidget {
  const _ColumnDropdown({
    required this.label,
    required this.value,
    required this.headers,
    required this.onChanged,
    this.optional = false,
  });

  final String label;
  final String? value;
  final List<String> headers;
  final ValueChanged<String?> onChanged;
  final bool optional;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: DropdownButtonFormField<String>(
        initialValue: value,
        decoration: InputDecoration(labelText: label),
        items: [
          if (optional) const DropdownMenuItem(value: '', child: Text('使用しない')),
          ...headers.map(
            (header) => DropdownMenuItem(value: header, child: Text(header)),
          ),
        ],
        onChanged: (newValue) =>
            onChanged(newValue == null || newValue.isEmpty ? null : newValue),
      ),
    );
  }
}

class _Preview extends StatelessWidget {
  const _Preview({
    required this.preview,
    required this.onCommit,
    required this.loading,
  });

  final ImportPreview preview;
  final VoidCallback onCommit;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Card(
          child: Padding(
            padding: const EdgeInsets.all(18),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'インポート前プレビュー',
                  style: TextStyle(fontWeight: FontWeight.w800, fontSize: 18),
                ),
                const SizedBox(height: 12),
                Text('件数: ${preview.readCount}件'),
                Text('対象期間: ${preview.periodStart} 〜 ${preview.periodEnd}'),
                Text('支出合計: ${yen(preview.expenseTotal)}'),
                Text('文字コード: ${preview.detectedEncoding}'),
                const Divider(height: 24),
                Text(
                  '日付=${preview.mapping.dateColumn} / '
                  '摘要=${preview.mapping.descriptionColumn} / '
                  '金額=${preview.mapping.amountColumn.isEmpty ? '${preview.mapping.debitColumn}・${preview.mapping.creditColumn}' : preview.mapping.amountColumn}',
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 12),
        Card(
          child: Column(
            children: [
              for (final row in preview.rows)
                ListTile(
                  title: Text(row.description),
                  subtitle: Text('${row.transactionDate}・${row.category}'),
                  trailing: Text(yen(row.amount)),
                ),
            ],
          ),
        ),
        if (preview.warnings.isNotEmpty) ...[
          const SizedBox(height: 10),
          Text(
            '${preview.warnings.length}件の行を読み飛ばしました',
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
        const SizedBox(height: 16),
        FilledButton.icon(
          key: const ValueKey('commit-import'),
          onPressed: loading ? null : onCommit,
          icon: const Icon(Icons.save_outlined),
          label: const Padding(
            padding: EdgeInsets.symmetric(vertical: 13),
            child: Text('この内容を保存'),
          ),
        ),
      ],
    );
  }
}
