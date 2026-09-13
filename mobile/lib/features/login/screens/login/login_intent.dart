import '../../../../shared/presentation/mvi.dart';

sealed class LoginIntent implements MviIntent {
  const LoginIntent();
}

class SubmitLogin extends LoginIntent {
  const SubmitLogin(this.username, this.password);
  final String username;
  final String password;
}
