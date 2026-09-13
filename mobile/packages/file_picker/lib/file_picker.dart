import 'package:flutter/services.dart';

enum FileType { any, custom }

class PlatformFile {
  const PlatformFile({
    required this.name,
    required this.size,
    this.path,
  });

  final String name;
  final int size;
  final String? path;
}

class FilePickerResult {
  const FilePickerResult(this.files);

  final List<PlatformFile> files;
}

class FilePicker {
  FilePicker._();

  static final FilePicker platform = FilePicker._();
  static const _channel = MethodChannel('spendable_today/file_picker');

  Future<FilePickerResult?> pickFiles({
    FileType type = FileType.any,
    List<String>? allowedExtensions,
    bool allowMultiple = false,
    bool withData = false,
  }) async {
    final result = await _channel.invokeMapMethod<String, Object?>(
      'pickFiles',
      <String, Object?>{
        'allowedExtensions': allowedExtensions ?? const <String>[],
        'allowMultiple': allowMultiple,
      },
    );
    if (result == null) {
      return null;
    }
    return FilePickerResult(<PlatformFile>[
      PlatformFile(
        name: result['name']! as String,
        path: result['path'] as String?,
        size: (result['size']! as num).toInt(),
      ),
    ]);
  }
}
