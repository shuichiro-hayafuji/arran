import '../../domain/session.dart';

class StoredSessionDto {
  static Session fromJson(Map<String, dynamic> json) => Session(
    token: json['access_token'] as String,
    expiresAt: DateTime.parse(json['expires_at'] as String),
  );

  static Map<String, dynamic> toJson(Session session) => {
    'access_token': session.token,
    'expires_at': session.expiresAt.toUtc().toIso8601String(),
  };
}
