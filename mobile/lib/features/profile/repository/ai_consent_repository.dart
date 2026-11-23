import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';

final aiConsentRepositoryProvider = Provider<AiConsentRepository>(
  (ref) => AiConsentRepository(ref.watch(authorizedApiClientProvider)),
);

class AiConsentStatus {
  const AiConsentStatus({
    required this.version,
    required this.description,
    required this.granted,
    required this.pending,
  });
  final String version;
  final String description;
  final bool granted;
  final bool pending;
}

class AiConsentRepository {
  const AiConsentRepository(this._client);
  final ApiClient _client;

  Future<AiConsentStatus> get() async {
    final response = await _client.get('/ai-consent');
    if (response.statusCode != 200) {
      throw const FormatException('同意の説明を取得できません。');
    }
    final data = parseMap(response.data);
    final version = data['version'];
    final description = data['description'];
    if (version is! String ||
        version.isEmpty ||
        description is! String ||
        description.trim().isEmpty ||
        data['granted'] is! bool ||
        data['withdrawal_pending'] is! bool) {
      throw const FormatException('同意の説明を取得できません。');
    }
    return AiConsentStatus(
      version: version,
      description: description,
      granted: data['granted'] as bool,
      pending: data['withdrawal_pending'] as bool,
    );
  }

  Future<void> grant(String version) async {
    final response = await _client.put(
      '/ai-consent',
      data: {'version': version},
    );
    if (response.statusCode != 200 ||
        parseMap(response.data)['status'] != 'granted') {
      throw const FormatException('同意の保存結果を確認できません。');
    }
  }

  Future<void> withdraw() async {
    final response = await _client.delete('/ai-consent');
    if (response.statusCode != 200 ||
        parseMap(response.data)['status'] != 'withdrawn') {
      throw const FormatException('同意の撤回結果を確認できません。');
    }
  }
}

final aiConsentStatusProvider = FutureProvider.autoDispose<AiConsentStatus>(
  (ref) => ref.watch(aiConsentRepositoryProvider).get(),
);
