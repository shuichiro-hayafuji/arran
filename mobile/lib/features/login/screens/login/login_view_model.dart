import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/network/api_exception.dart';
import '../../../../shared/presentation/mvi.dart';
import '../../../session/provider/session_provider.dart';
import '../../repository/login_repository.dart';
import 'login_intent.dart';
import 'login_state.dart';

final loginViewModelProvider =
    StateNotifierProvider.autoDispose<LoginViewModel, LoginState>(
      (ref) => LoginViewModel(
        ref.watch(loginRepositoryProvider),
        ref.watch(sessionControllerProvider),
      ),
    );

class LoginViewModel extends MviViewModel<LoginState, LoginIntent> {
  LoginViewModel(this._repository, this._session) : super(const LoginState());
  final LoginRepository _repository;
  final SessionController _session;

  @override
  Future<void> dispatch(LoginIntent intent) async {
    if (state.busy) return;
    switch (intent) {
      case SubmitLogin(:final username, :final password):
        if (username.trim().isEmpty || password.isEmpty) {
          state = const LoginState(error: 'ユーザー名とパスワードを入力してください。');
          return;
        }
        state = const LoginState(busy: true);
        final generation = _session.generation;
        try {
          final session = await _repository.login(username, password);
          if (!mounted || generation != _session.generation) {
            await _session.revoke(session);
            return;
          }
          await _session.activate(session);
          if (mounted) state = const LoginState();
        } on ApiException catch (error) {
          if (mounted) state = LoginState(error: error.message);
        } catch (_) {
          if (mounted)
            state = const LoginState(
              error: 'ログイン処理に失敗しました。アプリを再起動して再試行してください。',
            );
        } finally {
          if (mounted && state.busy) state = const LoginState();
        }
    }
  }
}
