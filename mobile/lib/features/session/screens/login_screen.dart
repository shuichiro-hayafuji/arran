import 'package:flutter/material.dart';
import '../../../core/network/api_exception.dart';
import '../provider/session_provider.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});
  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    authSession.addListener(_sessionChanged);
  }

  void _sessionChanged() {
    if (mounted) setState(() {});
  }

  Future<void> _login() async {
    if (_busy) return;
    if (_username.text.trim().isEmpty || _password.text.isEmpty) {
      setState(() => _error = 'ユーザー名とパスワードを入力してください。');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await authSession.login(_username.text, _password.text);
    } on ApiException catch (error) {
      if (mounted) setState(() => _error = error.message);
    } catch (_) {
      if (mounted) {
        setState(() => _error = 'ログイン処理に失敗しました。アプリを再起動して再試行してください。');
      }
    } finally {
      if (mounted) {
        _password.clear();
        setState(() => _busy = false);
      }
    }
  }

  @override
  void dispose() {
    authSession.removeListener(_sessionChanged);
    _username.dispose();
    _password.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!authSession.initialized) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    return Scaffold(
      appBar: AppBar(title: const Text('ログイン')),
      body: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: AutofillGroup(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text('管理者が登録したアカウントでログインしてください。'),
                  const SizedBox(height: 24),
                  TextField(
                    controller: _username,
                    enabled: !_busy,
                    decoration: const InputDecoration(labelText: 'ユーザー名'),
                    autofillHints: const [AutofillHints.username],
                    autocorrect: false,
                    textCapitalization: TextCapitalization.none,
                  ),
                  const SizedBox(height: 16),
                  TextField(
                    controller: _password,
                    enabled: !_busy,
                    obscureText: true,
                    decoration: const InputDecoration(labelText: 'パスワード'),
                    autofillHints: const [AutofillHints.password],
                    enableSuggestions: false,
                    autocorrect: false,
                    onSubmitted: (_) => _login(),
                  ),
                  const SizedBox(height: 16),
                  if (_error ?? authSession.storageError
                      case final String message)
                    Text(
                      message,
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.error,
                      ),
                    ),
                  const SizedBox(height: 16),
                  FilledButton(
                    onPressed: _busy ? null : _login,
                    child: Text(_busy ? 'ログイン中…' : 'ログイン'),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
