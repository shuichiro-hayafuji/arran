import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../repository/ai_consent_repository.dart';
import '../../../session/provider/session_provider.dart';

class AiConsentCard extends ConsumerStatefulWidget {
  const AiConsentCard({super.key});
  @override
  ConsumerState<AiConsentCard> createState() => _AiConsentCardState();
}

class _AiConsentCardState extends ConsumerState<AiConsentCard> {
  bool busy = false;
  String? error;
  Future<void> change(AiConsentStatus? status, bool grant) async {
    final generation = authSession.generation;
    setState(() {
      busy = true;
      error = null;
    });
    try {
      final repository = ref.read(aiConsentRepositoryProvider);
      if (grant) {
        await repository.grant(status!.version);
      } else {
        await repository.withdraw();
      }
      if (!mounted || authSession.generation != generation) return;
      ref.invalidate(aiConsentStatusProvider);
    } catch (_) {
      if (!mounted || authSession.generation != generation) return;
      setState(() {
        error = '操作結果を確認できません。再取得した状態が後から変わる可能性があります。';
      });
      ref.invalidate(aiConsentStatusProvider);
    } finally {
      if (mounted && authSession.generation == generation)
        setState(() {
          busy = false;
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(aiConsentStatusProvider);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('外部AIへの送信', style: Theme.of(context).textTheme.titleMedium),
            state.when(
              skipLoadingOnRefresh: false,
              skipLoadingOnReload: false,
              loading: () => const Padding(
                padding: EdgeInsets.all(12),
                child: CircularProgressIndicator(),
              ),
              error: (_, __) => Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text('同意の説明と状態を現在確認できません。'),
                  if (error != null) Text(error!),
                  TextButton(
                    onPressed: () => ref.invalidate(aiConsentStatusProvider),
                    child: const Text('再読み込み'),
                  ),
                  OutlinedButton(
                    onPressed: busy ? null : () => change(null, false),
                    child: const Text('同意を撤回する'),
                  ),
                ],
              ),
              data: (status) => Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const SizedBox(height: 8),
                  Text(status.description),
                  const SizedBox(height: 8),
                  Text(
                    status.pending
                        ? '撤回を処理中です。'
                        : status.granted
                        ? 'この説明に同意しています。'
                        : 'まだ同意していません。',
                  ),
                  if (error != null) Text(error!),
                  if (!status.granted && !status.pending)
                    FilledButton(
                      onPressed: busy ? null : () => change(status, true),
                      child: const Text('説明を確認して同意する'),
                    ),
                  if (status.granted || status.pending)
                    OutlinedButton(
                      onPressed: busy ? null : () => change(status, false),
                      child: const Text('同意を撤回する'),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
