import '../../../session/domain/session.dart';

class LoginResponseDto {
  LoginResponseDto.fromJson(Map<String, dynamic> json)
    : token = json['access_token'] as String,
      expiresAt = DateTime.parse(json['expires_at'] as String);

  final String token;
  final DateTime expiresAt;

  Session toDomain() => Session(token: token, expiresAt: expiresAt);
}
