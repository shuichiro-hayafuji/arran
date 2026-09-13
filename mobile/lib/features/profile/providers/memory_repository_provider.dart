import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/network/api_client.dart';
import '../repository/memory_repository.dart';

@riverpod
MemoryRepository memoryRepository(Ref ref) =>
    MemoryRepositoryImpl(ref.watch(authorizedApiClientProvider));

final memoryRepositoryProvider = Provider<MemoryRepository>(memoryRepository);
