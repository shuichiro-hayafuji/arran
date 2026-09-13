import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../screens/import/import_view_model.dart';

@riverpod
ImportViewModel importViewModelFactory(Ref ref) => ImportViewModel(ref);

final importControllerProvider =
    StateNotifierProvider<ImportViewModel, ImportState>(importViewModelFactory);
