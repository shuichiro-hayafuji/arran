import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../../../config/env.dart';
import '../../../core/network/api_client.dart';
import '../../session/provider/session_provider.dart';

typedef PendingImportOwnerResolver = Future<int> Function();

final pendingImportOwnerResolverProvider = Provider<PendingImportOwnerResolver>(
  (ref) {
    final session = ref.read(sessionControllerProvider);
    final token = session.token;
    final generation = session.generation;
    return () async {
      if (token == null) throw StateError('Authentication required');
      if (session.token != token || session.generation != generation) {
        throw StateError('Session changed');
      }
      final response = await ref
          .read(authorizedApiClientProvider)
          .get('/auth/me');
      if (session.token != token || session.generation != generation) {
        throw StateError('Session changed');
      }
      final data = response.data;
      if (response.statusCode != 200 ||
          data is! Map ||
          data['id'] is! int ||
          (data['id'] as int) <= 0) {
        throw const FormatException('Invalid authenticated user');
      }
      return data['id'] as int;
    };
  },
);

abstract interface class PendingImportStorage {
  Future<String?> load(int? ownerId);
  Future<void> save(int? ownerId, String previewId);
  Future<void> clear(int? ownerId, String previewId);
}

final pendingImportStorageProvider = Provider<PendingImportStorage>((ref) {
  final session = ref.read(sessionControllerProvider);
  final token = session.token;
  final generation = session.generation;
  return token == null
      ? MemoryPendingImportStorage()
      : SecurePendingImportStorage(
          () => session.token == token && session.generation == generation,
        );
});

class MemoryPendingImportStorage implements PendingImportStorage {
  String? value;
  @override
  Future<String?> load(int? ownerId) async => value;
  @override
  Future<void> save(int? ownerId, String previewId) async {
    if (value != null && value != previewId)
      throw StateError('Pending import exists');
    value = previewId;
  }

  @override
  Future<void> clear(int? ownerId, String previewId) async {
    if (value != null && value != previewId)
      throw StateError('Pending import changed');
    value = null;
  }
}

abstract interface class PendingImportVault {
  Future<String?> read(int ownerId);
  Future<void> write(int ownerId, String value);
  Future<void> delete(int ownerId);
}

class SecurePendingImportVault implements PendingImportVault {
  static const _storage = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
    iOptions: IOSOptions(
      accessibility: KeychainAccessibility.unlocked_this_device,
    ),
  );
  String _key(int ownerId) =>
      'arran.pending-import.v2.${Env.apiBaseUrl}.$ownerId';
  @override
  Future<String?> read(int ownerId) => _storage.read(key: _key(ownerId));
  @override
  Future<void> write(int ownerId, String value) =>
      _storage.write(key: _key(ownerId), value: value);
  @override
  Future<void> delete(int ownerId) => _storage.delete(key: _key(ownerId));
}

/// Serialize vault operations and fence stale session writes after every await.
class SecurePendingImportStorage implements PendingImportStorage {
  SecurePendingImportStorage(this.isCurrent, {PendingImportVault? vault})
    : _vault = vault ?? SecurePendingImportVault();
  final bool Function() isCurrent;
  final PendingImportVault _vault;
  int? _boundOwner;
  static Future<void> _queue = Future.value();

  Future<T> _serialized<T>(Future<T> Function() action) {
    final next = _queue.then((_) => action());
    _queue = next.then<void>((_) {}, onError: (Object _) {});
    return next;
  }

  int _check(int? ownerId) {
    if (ownerId == null || ownerId <= 0 || !isCurrent()) {
      throw StateError('Session changed or owner unavailable');
    }
    if (_boundOwner != null && _boundOwner != ownerId) {
      throw StateError('Owner changed');
    }
    _boundOwner = ownerId;
    return ownerId;
  }

  String _parse(String raw) {
    final value = jsonDecode(raw);
    if (value is! Map ||
        value['preview_id'] is! String ||
        (value['preview_id'] as String).isEmpty) {
      throw const FormatException('Invalid pending import');
    }
    return value['preview_id'] as String;
  }

  @override
  Future<String?> load(int? ownerId) => _serialized(() async {
    final owner = _check(ownerId);
    final raw = await _vault.read(owner);
    _check(ownerId);
    return raw == null ? null : _parse(raw);
  });

  @override
  Future<void> save(int? ownerId, String previewId) => _serialized(() async {
    final owner = _check(ownerId);
    final raw = await _vault.read(owner);
    _check(ownerId);
    if (raw != null && _parse(raw) != previewId)
      throw StateError('Pending import exists');
    await _vault.write(owner, jsonEncode({'preview_id': previewId}));
    _check(ownerId);
  });

  @override
  Future<void> clear(int? ownerId, String previewId) => _serialized(() async {
    final owner = _check(ownerId);
    final raw = await _vault.read(owner);
    _check(ownerId);
    if (raw == null) return;
    if (_parse(raw) != previewId) throw StateError('Pending import changed');
    await _vault.delete(owner);
    _check(ownerId);
  });
}
