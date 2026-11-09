import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'management_view_model.dart';

class ImportHistoryScreen extends ConsumerStatefulWidget {
  const ImportHistoryScreen({super.key});
  @override
  ConsumerState<ImportHistoryScreen> createState() =>
      _ImportHistoryScreenState();
}

class _ImportHistoryScreenState extends ConsumerState<ImportHistoryScreen> {
  String before = '';
  @override
  Widget build(BuildContext context) {
    final history = ref.watch(importHistoryProvider(before));
    return Scaffold(
      appBar: AppBar(title: const Text('CSV取込履歴')),
      body: history.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(error.toString()),
              TextButton(
                onPressed: () => ref.invalidate(importHistoryProvider(before)),
                child: const Text('読み直す'),
              ),
            ],
          ),
        ),
        data: (batches) => ListView(
          padding: const EdgeInsets.all(20),
          children: [
            const Text(
              '取消・訂正は銀行明細の表示と集計を変更します。相談の残高・自由予算は、次の相談で包含を確認してから更新します。',
            ),
            for (final batch in batches)
              ListTile(
                title: Text(
                  batch.fileName.isEmpty ? 'ファイル名不明' : batch.fileName,
                ),
                subtitle: Text(
                  '${batch.recordedAt.isEmpty ? '取込日時不明' : batch.recordedAt}\n${batch.source} / ${batch.account} • ${batch.status == 'active' ? '有効' : '取消済み'}',
                ),
                onTap: () => context.push('/imports/${batch.id}'),
              ),
            if (batches.length == 20)
              TextButton(
                onPressed: () => setState(() => before = batches.last.id),
                child: const Text('前の取込を見る'),
              ),
            if (before.isNotEmpty)
              TextButton(
                onPressed: () => setState(() => before = ''),
                child: const Text('最新に戻る'),
              ),
            if (batches.isEmpty)
              const Text('取込履歴はありません。過去の取込単位が分からない明細は、取引詳細から個別に訂正できます。'),
          ],
        ),
      ),
    );
  }
}
