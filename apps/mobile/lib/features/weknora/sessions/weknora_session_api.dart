import 'package:dio/dio.dart';

/// Thrown when the WeKnora server reports that a session no longer exists
/// (HTTP 404 on the message-history endpoint).
class WeKnoraSessionNotFoundException implements Exception {
  const WeKnoraSessionNotFoundException(this.sessionId);

  final String sessionId;

  @override
  String toString() => 'WeKnoraSessionNotFoundException: $sessionId';
}

/// Thrown when the WeKnora session endpoints answer with any non-2xx status
/// other than 404. Without this, a 401/500 would silently parse into an empty
/// list and look like "no history on the server".
class WeKnoraSessionApiException implements Exception {
  const WeKnoraSessionApiException({
    required this.statusCode,
    required this.message,
  });

  /// The HTTP status the server answered with; null when unavailable.
  final int? statusCode;
  final String message;

  @override
  String toString() => 'WeKnoraSessionApiException: $statusCode $message';
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
        validateStatus: (status) => true,
      ),
    );
    _ensureSuccess(response);
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
          validateStatus: (status) => true,
        ),
      );
      if (response.statusCode == 404) {
        throw WeKnoraSessionNotFoundException(sessionId);
      }
      _ensureSuccess(response);
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

  /// Maps any non-2xx response to [WeKnoraSessionApiException]. 404 handling
  /// is endpoint-specific and done by the callers, so it is not special-cased
  /// here.
  void _ensureSuccess(Response<dynamic> response) {
    final statusCode = response.statusCode;
    if (statusCode != null && statusCode >= 200 && statusCode < 300) return;
    throw WeKnoraSessionApiException(
      statusCode: statusCode,
      message: _serverErrorMessage(response.data),
    );
  }

  /// Best-effort extraction of the server's own error text from the common
  /// `{"error": ".."}` / `{"message": ".."}` / `{"detail": ".."}` bodies.
  String _serverErrorMessage(Object? body) {
    if (body is Map) {
      for (final key in const ['error', 'message', 'detail']) {
        final value = body[key];
        if (value is String && value.isNotEmpty) return value;
      }
    }
    return 'WeKnora request failed';
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
