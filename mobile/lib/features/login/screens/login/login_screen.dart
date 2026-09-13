import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/constants/space_theme.dart';
import '../../../session/provider/session_provider.dart';
import 'login_intent.dart';
import 'login_view_model.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});
  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  Future<void> _login() async {
    final viewModel = ref.read(loginViewModelProvider.notifier);
    if (ref.read(loginViewModelProvider).busy) return;
    await viewModel.dispatch(SubmitLogin(_username.text, _password.text));
    if (mounted) _password.clear();
  }

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(loginViewModelProvider);
    final session = ref.watch(sessionControllerProvider);
    return ListenableBuilder(
      listenable: session,
      builder: (context, _) {
        if (!session.initialized) {
          return const Scaffold(
            body: Center(child: CircularProgressIndicator()),
          );
        }
        return Scaffold(
          appBar: AppBar(title: const Text('ログイン')),
          body: Center(
            child: SingleChildScrollView(
              padding: AppSpace.pXxl,
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 420),
                child: AutofillGroup(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Text('管理者が登録したアカウントでログインしてください。'),
                      AppSpace.syXxl,
                      TextField(
                        controller: _username,
                        enabled: !state.busy,
                        decoration: const InputDecoration(labelText: 'ユーザー名'),
                        autofillHints: const [AutofillHints.username],
                        autocorrect: false,
                        textCapitalization: TextCapitalization.none,
                      ),
                      AppSpace.syXl,
                      TextField(
                        controller: _password,
                        enabled: !state.busy,
                        obscureText: true,
                        decoration: const InputDecoration(labelText: 'パスワード'),
                        autofillHints: const [AutofillHints.password],
                        enableSuggestions: false,
                        autocorrect: false,
                        onSubmitted: (_) => _login(),
                      ),
                      AppSpace.syXl,
                      if (state.error ?? session.storageError
                          case final String message)
                        Text(
                          message,
                          style: TextStyle(
                            color: Theme.of(context).colorScheme.error,
                          ),
                        ),
                      AppSpace.syXl,
                      FilledButton(
                        onPressed: state.busy ? null : _login,
                        child: Text(state.busy ? 'ログイン中…' : 'ログイン'),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}
