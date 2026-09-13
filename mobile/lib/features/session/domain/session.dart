import 'package:flutter/foundation.dart';

@immutable
class Session {
  const Session({required this.token, required this.expiresAt});

  factory Session.fromJson(Map<String, dynamic> json) => Session(
    token: json['access_token'] as String,
    expiresAt: DateTime.parse(json['expires_at'] as String),
  );

  final String token;
  final DateTime expiresAt;
  bool get alive => token.isNotEmpty && DateTime.now().isBefore(expiresAt);
  Map<String, dynamic> toJson() => {
    'access_token': token,
    'expires_at': expiresAt.toUtc().toIso8601String(),
  };
}
