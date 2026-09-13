import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../../../shared/utils/json_parsing.dart';
import '../domain/memory_item.dart';
import 'dto/memory_item_dto.dart';

@riverpod
MemoryRepository memoryRepository(Ref ref) =>
    MemoryRepositoryImpl(ref.watch(authorizedApiClientProvider));

final memoryRepositoryProvider = Provider<MemoryRepository>(memoryRepository);

abstract interface class MemoryRepository {
  Future<List<MemoryItem>> getAll();
  Future<MemoryItem> save(MemoryItem item);
  Future<void> delete(int id);
}

class MemoryRepositoryImpl implements MemoryRepository {
  const MemoryRepositoryImpl(this._client);

  final ApiClient _client;

  @override
  Future<List<MemoryItem>> getAll() async {
    final response = await _client.get('/memories');
    return parseMaps(
      parseMap(response.data)['items'],
    ).map(MemoryItemDto.fromJson).map((dto) => dto.toDomain()).toList();
  }

  @override
  Future<MemoryItem> save(MemoryItem item) async {
    final dto = MemoryItemDto.fromDomain(item);
    final response = item.id == 0
        ? await _client.post('/memories', data: dto.toJson())
        : await _client.patch('/memories/${item.id}', data: dto.toJson());
    return MemoryItemDto.fromJson(parseMap(response.data)).toDomain();
  }

  @override
  Future<void> delete(int id) async {
    await _client.delete('/memories/$id');
  }
}
