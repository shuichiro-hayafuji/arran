import 'purchase_csv_form.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:spendable_today/features/session/provider/session_provider.dart';
import 'package:spendable_today/features/transactions/domain/csv_mapping.dart';
import 'package:spendable_today/features/transactions/domain/import_preview.dart';
import 'package:spendable_today/features/transactions/screens/import/import_intent.dart';
import 'package:spendable_today/features/transactions/screens/import/import_view_model.dart';
import 'package:spendable_today/shared/utils/formatters.dart';
import '../../../../app/constants/space_theme.dart';

final importFilePickerProvider = Provider<Future<FilePickerResult?> Function()>(
  (ref) =>
      () => FilePicker.platform.pickFiles(
        type: FileType.custom,
        allowedExtensions: const ['csv'],
      ),
);

class ImportScreen extends ConsumerStatefulWidget {
  const ImportScreen({super.key});
  @override
  ConsumerState<ImportScreen> createState() => _ImportScreenState();
}

class _ImportScreenState extends ConsumerState<ImportScreen> {
  late final TextEditingController _source;
  late final TextEditingController _account;
  bool _allowPop = false;
  bool _confirmingLeave = false;
  @override
  void initState() {
    super.initState();
    final s = ref.read(importControllerProvider);
    _source = TextEditingController(
      text: s.source == "manual_csv" ? "手動CSV" : s.source,
    );
    _account = TextEditingController(text: s.accountName);
  }

  @override
  void dispose() {
    _source.dispose();
    _account.dispose();
    super.dispose();
  }

  String get sourceValue =>
      _source.text.trim() == "手動CSV" ? "manual_csv" : _source.text.trim();
  bool get dirty =>
      sourceValue != ref.read(importControllerProvider).source ||
      _account.text.trim() != ref.read(importControllerProvider).accountName;
  Future<void> _applySource() async {
    await ref
        .read(importControllerProvider.notifier)
        .dispatch(SetImportSource(sourceValue, _account.text));
    if (mounted) setState(() {});
  }

  Future<void> _pick() async {
    final session = ref.read(sessionControllerProvider);
    final token = session.token;
    final generation = session.generation;
    bool stillCurrent() =>
        mounted && session.token == token && session.generation == generation;
    final current = ref.read(importControllerProvider);
    if (current.commitOutcomeUnknown ||
        current.restoring ||
        current.recoveryBlocked ||
        current.cleanupRejected)
      return;
    if (dirty) {
      await _applySource();
      if (!stillCurrent() ||
          ref.read(importControllerProvider).source != sourceValue ||
          ref.read(importControllerProvider).accountName !=
              _account.text.trim())
        return;
    }
    if (!stillCurrent()) return;
    final result = await ref.read(importFilePickerProvider)();
    if (!stillCurrent()) return;
    if (result == null || result.files.isEmpty) return;
    final file = result.files.first;
    if (file.path == null) return;
    await ref
        .read(importControllerProvider.notifier)
        .dispatch(SelectImportFile(filePath: file.path!, fileName: file.name));
  }

  Future<void> _confirmLeave() async {
    if (_confirmingLeave) return;
    _confirmingLeave = true;
    final rejected = ref.read(importControllerProvider).cleanupRejected;
    final leave = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(rejected ? '確認情報を削除できていません' : '保存結果を確認できていません'),
        content: Text(
          rejected
              ? '保存は拒否されました。端末の確認情報を削除してから次の取込を始めてください。'
              : '別の画面から戻った場合や、同じ利用者で再ログインした場合は確認を試みます。'
                    '別の利用者には確認情報を表示しません。'
                    '結果が分かるまで新しく取り込まず、必要なら運用者に確認してください。',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('確認を続ける'),
          ),
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('別の画面へ移動'),
          ),
        ],
      ),
    );
    _confirmingLeave = false;
    if (leave != true || !mounted) return;
    setState(() => _allowPop = true);
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (!mounted) return;
      final didPop = await Navigator.of(context).maybePop();
      if (!didPop && mounted) setState(() => _allowPop = false);
    });
  }

  Future<void> _confirmOperatorRecovery() async {
    final id = ref.read(importControllerProvider).pendingPreviewId;
    if (id == null) return;
    var confirmation = '';
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('運用者による確認が必要です'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                SelectableText('確認対象のプレビューID: $id'),
                const Text(
                  '運用者に再送の停止、新たな確定の防止、受付済み処理がすべて終了したことを確認してから、未保存か確認してもらってください。処理終了を確認できない場合は再開しないでください。アプリには処理終了を確認する機能がありません。期限切れやエラーだけでは未保存と判断できず、再取込すると重複するおそれがあります。',
                ),
                const Text('運用者から未保存と確認できた場合のみ「未保存を確認しました」と入力してください。'),
                TextField(
                  onChanged: (value) =>
                      setDialogState(() => confirmation = value),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('戻る'),
            ),
            TextButton(
              onPressed: confirmation == '未保存を確認しました'
                  ? () => Navigator.pop(dialogContext, true)
                  : null,
              child: const Text('確認情報を削除して再開'),
            ),
          ],
        ),
      ),
    );
    if (confirmed == true && mounted) {
      await ref
          .read(importControllerProvider.notifier)
          .resumeAfterOperatorConfirmation();
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(importControllerProvider);
    final controller = ref.read(importControllerProvider.notifier);
    final editingBlocked =
        state.loading ||
        state.restoring ||
        state.recoveryBlocked ||
        state.cleanupRejected ||
        state.commitOutcomeUnknown ||
        state.commitResult != null;
    return PopScope(
      canPop:
          _allowPop ||
          !(state.commitOutcomeUnknown ||
              state.restoring ||
              state.recoveryBlocked ||
              state.cleanupRejected),
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop &&
            (state.commitOutcomeUnknown ||
                state.restoring ||
                state.recoveryBlocked ||
                state.cleanupRejected))
          _confirmLeave();
      },
      child: Scaffold(
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
            const Text('UTF-8、Shift_JIS/CP932に対応。確定するまで取引・集計には反映されません。'),
            if (state.commitOutcomeUnknown) ...[
              AppSpace.syM,
              const Text(
                '保存結果が不明です。新しく取り込まず、同じ内容で保存結果を確認してください。'
                '同じ利用者で再ログインした場合も確認情報の復元を試みます。'
                '別の利用者には確認情報を表示しません。',
              ),
              TextButton(
                onPressed: state.loading || state.recoveryBlocked
                    ? null
                    : _confirmOperatorRecovery,
                child: const Text('運用者に確認した場合の再開'),
              ),
            ],
            if (state.recoveryBlocked) ...[
              AppSpace.syM,
              TextButton(
                onPressed: state.restoring ? null : controller.retryRestore,
                child: const Text('確認情報を読み直す'),
              ),
            ],
            if (state.cleanupRejected) ...[
              AppSpace.syM,
              TextButton(
                onPressed: state.loading
                    ? null
                    : controller.retryRejectedCleanup,
                child: const Text('確認情報の削除を再試行'),
              ),
            ],
            AppSpace.syL,
            TextField(
              key: const ValueKey('import-source'),
              controller: _source,
              enabled: !editingBlocked,
              onChanged: (_) => setState(() {}),
              decoration: const InputDecoration(labelText: '取込元（銀行・カード名など）'),
            ),
            AppSpace.syM,
            TextField(
              key: const ValueKey('import-account'),
              controller: _account,
              enabled: !editingBlocked,
              onChanged: (_) => setState(() {}),
              decoration: const InputDecoration(
                labelText: '口座名',
                helperText: '同じ口座は再取込でも同じ名前を使ってください。',
              ),
            ),
            if (dirty)
              TextButton(
                onPressed: editingBlocked ? null : _applySource,
                child: const Text('取込元・口座を適用して再確認'),
              ),
            AppSpace.syL,
            OutlinedButton.icon(
              key: const ValueKey('pick-csv'),
              onPressed: editingBlocked ? null : () => _pick(),
              icon: const Icon(Icons.file_open_outlined),
              label: Padding(
                padding: const EdgeInsets.symmetric(vertical: 13),
                child: Text(state.fileName ?? 'CSVファイルを選択'),
              ),
            ),
            if (state.loading || state.restoring) ...[
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
            if (state.preview != null && !editingBlocked) ...[
              const SizedBox(height: 20),
              if (state.preview!.needsMapping)
                _MappingForm(
                  key: ValueKey(state.preview),
                  preview: state.preview!,
                  onSubmit: (mapping) =>
                      controller.dispatch(ApplyImportMapping(mapping)),
                )
              else
                ExpansionTile(
                  title: const Text('列の割当・明細ID（必要な場合）'),
                  children: [
                    _MappingForm(
                      key: ValueKey(state.preview),
                      preview: state.preview!,
                      onSubmit: (mapping) =>
                          controller.dispatch(ApplyImportMapping(mapping)),
                    ),
                  ],
                ),
            ],
            if (state.preview != null && !state.preview!.needsMapping) ...[
              const SizedBox(height: 20),
              _Preview(
                preview: state.preview!,
                onCommit: () => controller.dispatch(const CommitImport()),
                checkingResult: state.commitOutcomeUnknown,
                loading:
                    state.loading ||
                    state.restoring ||
                    state.recoveryBlocked ||
                    state.cleanupRejected ||
                    state.commitResult != null ||
                    state.preview!.previewId.isEmpty ||
                    dirty ||
                    state.preview!.identityConflicts.isNotEmpty ||
                    state.preview!.reconciliationConflicts.isNotEmpty,
              ),
            ],
            if (state.preview == null &&
                state.commitOutcomeUnknown &&
                state.pendingPreviewId != null) ...[
              AppSpace.syL,
              FilledButton(
                key: const ValueKey('commit-import'),
                onPressed: state.loading
                    ? null
                    : () => controller.dispatch(const CommitImport()),
                child: const Text('保存結果を確認'),
              ),
            ],
            if (state.preview?.identityConflicts.isNotEmpty == true) ...[
              AppSpace.syL,
              _IdentityForm(
                key: ValueKey(state.preview),
                preview: state.preview!,
                loading: editingBlocked || dirty,
                onSubmit: (values) =>
                    controller.dispatch(ResolveImportIdentities(values)),
              ),
            ],
            if (state.preview?.reconciliationConflicts.isNotEmpty == true) ...[
              AppSpace.syL,
              PurchaseCsvForm(
                key: ValueKey(state.preview),
                conflicts: state.preview!.reconciliationConflicts,
                loading: editingBlocked || dirty,
                onSubmit: (values) =>
                    controller.dispatch(ResolveImportPurchases(values)),
              ),
            ],
            if (state.preview?.reactivationIds.isNotEmpty == true &&
                !state.confirmReactivation) ...[
              const Text('取消済みの明細が含まれています。同じ明細と最新訂正を保持して再び有効にします。残高は自動変更しません。'),
              TextButton(
                onPressed: editingBlocked || dirty
                    ? null
                    : () => controller.dispatch(
                        const ConfirmImportReactivation(),
                      ),
                child: const Text('取消済み明細の再有効化を確認'),
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
                      if (state.commitResult!.reactivatedCount > 0)
                        Text('再有効化 ${state.commitResult!.reactivatedCount}件'),
                      if (state.commitResult!.importId.isNotEmpty)
                        TextButton(
                          onPressed: () => context.push(
                            '/imports/${state.commitResult!.importId}',
                          ),
                          child: const Text('この取込の履歴を見る'),
                        ),
                      const Text(
                        '購入との照合は保存済みです。残高・自由予算の反映に確認が必要な場合は、次のチャットで質問します。',
                      ),
                      const SizedBox(height: 12),
                      FilledButton(
                        onPressed: state.loading
                            ? null
                            : () async {
                                await controller.dispatch(const ResetImport());
                                if (context.mounted &&
                                    ref
                                            .read(importControllerProvider)
                                            .commitResult ==
                                        null) {
                                  context.go('/transactions');
                                }
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
      ),
    );
  }
}

class _MappingForm extends StatefulWidget {
  const _MappingForm({
    super.key,
    required this.preview,
    required this.onSubmit,
  });

  final ImportPreview preview;
  final ValueChanged<CsvMapping> onSubmit;

  @override
  State<_MappingForm> createState() => _MappingFormState();
}

enum _AmountMode { single, split }

class _MappingFormState extends State<_MappingForm> {
  late String? _date = _valid(widget.preview.mapping.dateColumn);
  late String? _description = _valid(widget.preview.mapping.descriptionColumn);
  late String? _amount = _valid(widget.preview.mapping.amountColumn);
  late String? _debit = _valid(widget.preview.mapping.debitColumn);
  late String? _credit = _valid(widget.preview.mapping.creditColumn);
  late String? _transactionId = _valid(
    widget.preview.mapping.transactionIdColumn,
  );

  late _AmountMode _mode = _amount != null
      ? _AmountMode.single
      : (_debit != null || _credit != null
            ? _AmountMode.split
            : _AmountMode.single);

  String? _valid(String value) =>
      value.isNotEmpty && widget.preview.headers.contains(value) ? value : null;

  @override
  void didUpdateWidget(covariant _MappingForm oldWidget) {
    super.didUpdateWidget(oldWidget);
    _date = _valid(_date ?? '');
    _description = _valid(_description ?? '');
    _amount = _valid(_amount ?? '');
    _debit = _valid(_debit ?? '');
    _credit = _valid(_credit ?? '');
    _transactionId = _valid(_transactionId ?? '');
  }

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
            const Text('残高ではなく、その取引の金額が入った列を選んでください。'),
            DropdownButtonFormField<_AmountMode>(
              key: const ValueKey('mapping-amount-mode'),
              isExpanded: true,
              initialValue: _mode,
              decoration: const InputDecoration(labelText: '金額の列の使い方'),
              items: const [
                DropdownMenuItem(
                  value: _AmountMode.single,
                  child: Text('金額1列'),
                ),
                DropdownMenuItem(
                  value: _AmountMode.split,
                  child: Text('出金・入金の別列'),
                ),
              ],
              onChanged: (mode) {
                if (mode != null) setState(() => _mode = mode);
              },
            ),
            AppSpace.syM,
            if (_mode == _AmountMode.single)
              _ColumnDropdown(
                label: '金額列',
                value: _amount,
                headers: headers,
                onChanged: (value) => setState(() => _amount = value),
              )
            else ...[
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
            ],
            _ColumnDropdown(
              label: '安定した明細ID列（取込元が付けたIDがある場合のみ）',
              value: _transactionId,
              headers: headers,
              optional: true,
              onChanged: (v) => setState(() => _transactionId = v),
            ),
            AppSpace.syS,
            FilledButton(
              key: const ValueKey('apply-mapping'),
              onPressed:
                  _date == null ||
                      _description == null ||
                      (_mode == _AmountMode.single
                          ? _amount == null
                          : (_debit == null && _credit == null))
                  ? null
                  : () => widget.onSubmit(
                      CsvMapping(
                        transactionIdColumn: _transactionId ?? '',
                        dateColumn: _date!,
                        descriptionColumn: _description!,
                        amountColumn: _mode == _AmountMode.single
                            ? _amount ?? ''
                            : '',
                        debitColumn: _mode == _AmountMode.split
                            ? _debit ?? ''
                            : '',
                        creditColumn: _mode == _AmountMode.split
                            ? _credit ?? ''
                            : '',
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
        key: ValueKey((label, value, headers.join('\u0000'))),
        isExpanded: true,
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
    required this.checkingResult,
  });

  final ImportPreview preview;
  final VoidCallback onCommit;
  final bool loading;
  final bool checkingResult;

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
                if (preview.mapping.transactionIdColumn.isNotEmpty)
                  Text('明細ID列: ${preview.mapping.transactionIdColumn}'),
                AppSpace.syS,
                Text(preview.identityNotice),
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
          label: Padding(
            padding: const EdgeInsets.symmetric(vertical: 13),
            child: Text(checkingResult ? '保存結果を確認' : 'この内容を保存'),
          ),
        ),
      ],
    );
  }
}

class _IdentityForm extends StatefulWidget {
  const _IdentityForm({
    super.key,
    required this.preview,
    required this.loading,
    required this.onSubmit,
  });
  final ImportPreview preview;
  final bool loading;
  final ValueChanged<List<ImportIdentityDecision>> onSubmit;
  @override
  State<_IdentityForm> createState() => _IdentityFormState();
}

class _IdentityFormState extends State<_IdentityForm> {
  final Map<int, int> selected = {};
  @override
  Widget build(BuildContext context) => Card(
    child: Padding(
      padding: AppSpace.pXl,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('同じ明細か確認してください', style: Theme.of(context).textTheme.titleMedium),
          const Text('別のCSVにも同条件の明細があります。入出金の回数を誤って増減しないよう、各行を確認してから保存します。'),
          for (final conflict in widget.preview.identityConflicts) ...[
            AppSpace.syL,
            Text(
              '${conflict.row}行目: ${conflict.transaction.transactionDate} ${conflict.transaction.transactionType == "income" ? "入金" : "支出"} ${yen(conflict.transaction.amount)} ${conflict.transaction.description}',
            ),
            DropdownButtonFormField<int>(
              key: ValueKey('identity-row-${conflict.row}'),
              isExpanded: true,
              decoration: const InputDecoration(labelText: 'この行の扱い'),
              items: [
                const DropdownMenuItem(value: 0, child: Text('別の新しい明細として保存')),
                for (final candidate in conflict.candidates)
                  DropdownMenuItem(
                    value: candidate.id,
                    child: Text(
                      '既存 #${candidate.id}: ${candidate.description}（${candidate.category}）と同じ',
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
              ],
              onChanged: widget.loading
                  ? null
                  : (v) => setState(() => selected[conflict.row] = v!),
            ),
          ],
          AppSpace.syL,
          FilledButton(
            key: const ValueKey('resolve-import-identities'),
            onPressed:
                widget.loading ||
                    selected.length != widget.preview.identityConflicts.length
                ? null
                : () => widget.onSubmit(
                    widget.preview.identityConflicts
                        .map(
                          (c) => ImportIdentityDecision(
                            row: c.row,
                            action: selected[c.row] == 0 ? 'distinct' : 'same',
                            transactionId: selected[c.row]!,
                            snapshot:
                                c.candidateSnapshots[selected[c.row]] ?? '',
                          ),
                        )
                        .toList(),
                  ),
            child: const Text('確認した内容で再プレビュー'),
          ),
        ],
      ),
    ),
  );
}
