import 'dart:convert';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../../../config/env.dart';
import '../../session/provider/session_provider.dart';
import '../domain/chat_request.dart';

abstract interface class PendingRequestStorage {
  Future<PendingChatRequest?> load(String? scope);
  Future<void> save(String? scope, PendingChatRequest value);
  Future<void> clear(String? scope);
}

final pendingRequestStorageProvider = Provider<PendingRequestStorage>((ref) {
  final session = ref.watch(sessionControllerProvider);
  return session.token == null
      ? MemoryPendingRequestStorage()
      : SecurePendingRequestStorage(() => session.token);
});

class MemoryPendingRequestStorage implements PendingRequestStorage {
  PendingChatRequest? value;
  @override
  Future<PendingChatRequest?> load(String? scope) async => value;
  @override
  Future<void> save(String? scope, PendingChatRequest request) async {
    value = request;
  }

  @override
  Future<void> clear(String? scope) async {
    value = null;
  }
}

// Serialized secure writes prevent a disposed session's late write/delete from
// replacing the new session's pending request. Scope is kept inside encrypted data.
class SecurePendingRequestStorage implements PendingRequestStorage {
  SecurePendingRequestStorage(this.currentScope);
  final String? Function() currentScope;
  static const _storage = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
    iOptions: IOSOptions(
      accessibility: KeychainAccessibility.unlocked_this_device,
    ),
  );
  static Future<void> _queue = Future.value();
  String get _key => 'arran.pending-chat.v1.${Env.apiBaseUrl}';
  Future<T> _serialized<T>(Future<T> Function() action) {
    final next = _queue.then((_) => action());
    _queue = next.then<void>((_) {}, onError: (Object _) {});
    return next;
  }

  @override
  Future<PendingChatRequest?> load(String? scope) => _serialized(() async {
    if (scope == null || currentScope() != scope) return null;
    final raw = await _storage.read(key: _key);
    if (raw == null) return null;
    final value = jsonDecode(raw) as Map<String, dynamic>;
    if (value['scope'] != scope) return null;
    return PendingChatRequest(
      id: value['id'] as String,
      message: value['message'] as String,
      consultationId: value['consultation_id'] as int,
      plannedAmount: value['planned_amount'] as int?,
      cancelRequested: value['cancel_requested'] as bool,
    );
  });
  @override
  Future<void> save(String? scope, PendingChatRequest value) =>
      _serialized(() async {
        if (scope == null || currentScope() != scope) return;
        await _storage.write(
          key: _key,
          value: jsonEncode({
            'scope': scope,
            'id': value.id,
            'message': value.message,
            'consultation_id': value.consultationId,
            'planned_amount': value.plannedAmount,
            'cancel_requested': value.cancelRequested,
          }),
        );
      });
  @override
  Future<void> clear(String? scope) => _serialized(() async {
    final raw = await _storage.read(key: _key);
    if (raw == null) return;
    final value = jsonDecode(raw) as Map<String, dynamic>;
    if (value['scope'] == scope) await _storage.delete(key: _key);
  });
}
