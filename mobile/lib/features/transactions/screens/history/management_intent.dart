import '../../../../shared/presentation/mvi.dart';
import '../../domain/import_management.dart';

sealed class ManagementIntent implements MviIntent {
  const ManagementIntent();
}

class LoadManagement extends ManagementIntent {
  const LoadManagement();
}

class SaveManagement extends ManagementIntent {
  const SaveManagement(this.request);
  final MutationRequest request;
}

class RecoverManagement extends ManagementIntent {
  const RecoverManagement();
}
