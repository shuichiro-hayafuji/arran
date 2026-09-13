import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:spendable_today/features/profile/domain/memory_item.dart';
import 'package:spendable_today/features/profile/screens/memories/memory_intent.dart';
import 'package:spendable_today/features/profile/providers/memories_provider.dart';
import 'package:spendable_today/shared/widgets/async_error_card.dart';

class MemoryScreen extends ConsumerWidget {
  const MemoryScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final memoryState = ref.watch(memoriesViewModelProvider);
    ref.listen(memoriesViewModelProvider, (previous, next) {
      if (next.actionError != null &&
          next.actionError != previous?.actionError &&
          context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(next.actionError.toString())));
      }
    });
    return Scaffold(
      appBar: AppBar(title: const Text('AIが参照するメモリ')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: memoryState.saving ? null : () => _edit(context, ref, null),
        icon: const Icon(Icons.add),
        label: const Text('追加'),
      ),
      body: memoryState.items.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Padding(
          padding: const EdgeInsets.all(20),
          child: AsyncErrorCard(
            error: error,
            onRetry: () => ref
                .read(memoriesViewModelProvider.notifier)
                .dispatch(const RefreshMemories()),
          ),
        ),
        data: (items) => items.isEmpty
            ? const Center(
                child: Padding(
                  padding: EdgeInsets.all(30),
                  child: Text('相談結果を記録すると、再利用できる価値観や後悔パターンがここに蓄積されます。'),
                ),
              )
            : ListView.separated(
                padding: const EdgeInsets.fromLTRB(20, 12, 20, 100),
                itemCount: items.length,
                separatorBuilder: (_, _) => const SizedBox(height: 10),
                itemBuilder: (context, index) {
                  final item = items[index];
                  return Card(
                    child: ListTile(
                      title: Text(item.content),
                      subtitle: Text('${item.type}\n根拠: ${item.evidence}'),
                      isThreeLine: true,
                      onTap: memoryState.saving
                          ? null
                          : () => _edit(context, ref, item),
                      trailing: IconButton(
                        tooltip: '削除',
                        icon: const Icon(Icons.delete_outline),
                        onPressed: memoryState.saving
                            ? null
                            : () => ref
                                  .read(memoriesViewModelProvider.notifier)
                                  .dispatch(DeleteMemory(item.id)),
                      ),
                    ),
                  );
                },
              ),
      ),
    );
  }

  Future<void> _edit(
    BuildContext context,
    WidgetRef ref,
    MemoryItem? item,
  ) async {
    final content = TextEditingController(text: item?.content ?? '');
    final evidence = TextEditingController(text: item?.evidence ?? '');
    var type = item?.type ?? 'value';
    final saved = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setState) => AlertDialog(
          title: Text(item == null ? 'メモリを追加' : 'メモリを編集'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                DropdownButtonFormField<String>(
                  initialValue: type,
                  decoration: const InputDecoration(labelText: '種類'),
                  items:
                      const [
                            'value',
                            'spending_preference',
                            'regret_pattern',
                            'positive_pattern',
                            'merchant_rule',
                            'goal',
                            'behavioral_tendency',
                          ]
                          .map(
                            (value) => DropdownMenuItem(
                              value: value,
                              child: Text(value),
                            ),
                          )
                          .toList(),
                  onChanged: (value) {
                    if (value != null) setState(() => type = value);
                  },
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: content,
                  minLines: 2,
                  maxLines: 4,
                  decoration: const InputDecoration(labelText: '内容'),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: evidence,
                  minLines: 2,
                  maxLines: 4,
                  decoration: const InputDecoration(labelText: '根拠'),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('キャンセル'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext, true),
              child: const Text('保存'),
            ),
          ],
        ),
      ),
    );
    if (saved == true && content.text.trim().isNotEmpty) {
      await ref
          .read(memoriesViewModelProvider.notifier)
          .dispatch(
            SaveMemory(
              MemoryItem(
                id: item?.id ?? 0,
                type: type,
                content: content.text.trim(),
                evidence: evidence.text.trim(),
                confidence: item?.confidence ?? 1,
              ),
            ),
          );
    }
    content.dispose();
    evidence.dispose();
  }
}
