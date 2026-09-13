import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../session/provider/session_provider.dart';

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
      children: [
        ListTile(
          leading: const Icon(Icons.logout),
          title: const Text('ログアウト'),
          onTap: () async {
            try {
              await authSession.logout();
            } catch (_) {
              if (context.mounted)
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(
                    content: Text('ログアウトできませんでした。接続を確認して再試行してください。'),
                  ),
                );
            }
          },
        ),
        Text(
          '設定',
          style: Theme.of(
            context,
          ).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w800),
        ),
        const SizedBox(height: 16),
        Card(
          child: Column(
            children: [
              ListTile(
                leading: const Icon(Icons.person_outline),
                title: const Text('プロフィールを編集'),
                trailing: const Icon(Icons.chevron_right),
                onTap: () => context.push('/profile'),
              ),
              const Divider(height: 1),
              ListTile(
                leading: const Icon(Icons.psychology_outlined),
                title: const Text('AIが参照するメモリ'),
                subtitle: const Text('確認・編集・削除'),
                trailing: const Icon(Icons.chevron_right),
                onTap: () => context.push('/memories'),
              ),
              const Divider(height: 1),
              ListTile(
                leading: const Icon(Icons.upload_file_outlined),
                title: const Text('CSVをインポート'),
                trailing: const Icon(Icons.chevron_right),
                onTap: () => context.push('/import'),
              ),
            ],
          ),
        ),
        const SizedBox(height: 20),
        Text(
          '接続先',
          style: Theme.of(
            context,
          ).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800),
        ),
        const SizedBox(height: 8),
        Card(
          child: ListTile(
            leading: const Icon(Icons.dns_outlined),
            subtitle: const Text('APIキーはモバイルアプリに保存しません'),
          ),
        ),
      ],
    );
  }
}
