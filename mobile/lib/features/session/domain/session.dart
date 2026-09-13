class Session {
  const Session({required this.token, required this.expiresAt});

  final String token;
  final DateTime expiresAt;
  bool get alive => token.isNotEmpty && DateTime.now().isBefore(expiresAt);
}
