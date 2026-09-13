int parseInt(Object? value) => (value as num?)?.toInt() ?? 0;

int? parseNullableInt(Object? value) => (value as num?)?.toInt();

List<String> parseStrings(Object? value) =>
    (value as List<dynamic>? ?? const []).whereType<String>().toList();

Map<String, dynamic> parseMap(Object? value) =>
    Map<String, dynamic>.from(value as Map? ?? const {});

List<Map<String, dynamic>> parseMaps(Object? value) =>
    (value as List<dynamic>? ?? const []).map(parseMap).toList();
