abstract final class Env {
  static const apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://127.0.0.1:8080',
  );
  static const connectTimeout = int.fromEnvironment(
    'API_TIMEOUT_MILLS',
    defaultValue: 5000
  );
  static const receiveTimeout = int.fromEnvironment(
    'API_RECEIVE_TIMEOUT_MILLS',
    defaultValue: 10000
  );
  static const sendTimeout = int.fromEnvironment(
    'API_SEND_TIMEOUT_MILLS',
    defaultValue: 3000
  );
}
