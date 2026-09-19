import 'package:dio/dio.dart';

/// Thrown when the WeKnora server reports that a session no longer exists
/// (HTTP 404 on the message-history endpoint).
class WeKnoraSessionNotFoundException implements Exception {
  const WeKnoraSessionNotFoundException(this.sessionId);

  final String sessionId;

  @override
  String toString() => 'WeKnoraSessionNotFoundException: $sessionId';
}

class WeKnoraSessionSummary {
  const WeKnoraSessionSummary({
    required this.id,
    required this.title,
    required this.updatedAt,
    required this.createdAt,
    required this.pinned,
  });

  final String id;
  final String title;
  final DateTime updatedAt;
  final DateTime createdAt;
  final bool pinned;
}

class WeKnoraHistoryMessage {
  const WeKnoraHistoryMessage({
    required this.role,
    required this.content,
    this.sourceLines = const [],
  });

  final String role;
  final String content;

  /// Markdown lines (`1. [title](url) — snippet`) folded from the backend's
  /// `knowledge_references` entries.
  final List<String> sourceLines;
}

class WeKnoraSessionApi {
  WeKnoraSessionApi({
    required Future<({String baseUrl, String token})> Function() connection,
    required Dio Function() dioFactory,
  }) : _connection = connection,
       _dioFactory = dioFactory;

  final Future<({String baseUrl, String token})> Function() _connection;
  final Dio Function() _dioFactory;

  Future<List<WeKnoraSessionSummary>> listSessions({int limit = 50}) async {
    final connection = await _connection();
    final dio = _dioFactory();
    _applyBaseUrl(dio, connection.baseUrl);
    final response = await dio.get<dynamic>(
      'api/v1/sessions',
      queryParameters: {'limit': limit},
      options: Options(
        headers: {'Authorization': 'Bearer ${connection.token}'},
      ),
    );
    return [
      for (final raw in _dataList(response.data))
        if (raw is Map) _parseSession(raw.cast<String, dynamic>()),
    ];
  }

  Future<List<WeKnoraHistoryMessage>> loadMessages({
    required String sessionId,
    int limit = 200,
  }) async {
    final connection = await _connection();
    final dio = _dioFactory();
    _applyBaseUrl(dio, connection.baseUrl);
    try {
      final response = await dio.get<dynamic>(
        'api/v1/messages/$sessionId/load',
        queryParameters: {'limit': limit},
        options: Options(
          headers: {'Authorization': 'Bearer ${connection.token}'},
          validateStatus: (status) => status != null && status < 500,
        ),
      );
      if (response.statusCode == 404) {
        throw WeKnoraSessionNotFoundException(sessionId);
      }
      return [
        for (final raw in _dataList(response.data))
          if (raw is Map) _parseHistoryMessage(raw.cast<String, dynamic>()),
      ];
    } on DioException catch (error) {
      // A 404 that reaches us as a DioException (adapter-level rejection
      // rather than a validated response) still means "session missing".
      if (error.response?.statusCode == 404) {
        throw WeKnoraSessionNotFoundException(sessionId);
      }
      rethrow;
    }
  }

  WeKnoraSessionSummary _parseSession(Map<String, dynamic> map) {
    return WeKnoraSessionSummary(
      id: map['id'] as String,
      title: _stringField(map['title']),
      updatedAt:
          DateTime.tryParse(_stringField(map['updated_at'])) ?? DateTime.now(),
      createdAt:
          DateTime.tryParse(_stringField(map['created_at'])) ?? DateTime.now(),
      pinned: map['is_pinned'] == true,
    );
  }

  WeKnoraHistoryMessage _parseHistoryMessage(Map<String, dynamic> map) {
    return WeKnoraHistoryMessage(
      role: _stringField(map['role']),
      content: _stringField(map['content']),
      sourceLines: _sourceLines(map['knowledge_references']),
    );
  }

  /// Folds `knowledge_references` entries into numbered markdown lines.
  /// Entries that are not maps are malformed and skipped; numbering stays
  /// dense across the lines that survive.
  List<String> _sourceLines(Object? references) {
    if (references is! List) return const [];
    final lines = <String>[];
    for (final entry in references) {
      if (entry is! Map) continue;
      final map = entry.cast<String, dynamic>();
      final title = _stringField(map['source_title']);
      final snippet = _stringField(map['knowledge_snippet']);
      final url = _stringField(map['source_url']);
      final index = lines.length + 1;
      if (url.toLowerCase().startsWith('http')) {
        lines.add('$index. [$title]($url) — $snippet');
      } else {
        lines.add('$index. $title — $snippet');
      }
    }
    return lines;
  }

  List<dynamic> _dataList(Object? body) {
    if (body is Map) {
      final data = body['data'];
      if (data is List) return data;
    }
    return const [];
  }

  String _stringField(Object? value) {
    if (value == null) return '';
    if (value is String) return value;
    return value.toString();
  }
}

void _applyBaseUrl(Dio dio, String baseUrl) {
  // Dio concatenates baseUrl and path verbatim, so the base needs its
  // trailing slash for relative paths such as 'api/v1/sessions' to resolve
  // (same idiom as WeKnoraAuthClient).
  final trimmed = baseUrl.trim();
  dio.options.baseUrl = trimmed.endsWith('/') ? trimmed : '$trimmed/';
}
