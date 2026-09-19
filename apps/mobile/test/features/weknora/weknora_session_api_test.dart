import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:conduit/features/weknora/sessions/weknora_session_api.dart';

void main() {
  WeKnoraSessionApi newApi(
    _QueuedAdapter http, {
    String baseUrl = 'http://localhost:8084',
    String token = 'at-1',
  }) {
    return WeKnoraSessionApi(
      connection: () async => (baseUrl: baseUrl, token: token),
      dioFactory: () => _dio(http),
    );
  }

  group('listSessions', () {
    test('parses data array and sends bearer token', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'data': [
            {
              'id': 's1',
              'title': 'RAG 问答',
              'updated_at': '2026-09-19T10:00:00Z',
              'created_at': '2026-09-18T09:00:00Z',
              'is_pinned': true,
              'last_request_state': {'mode': 'rag'},
            },
            {
              'id': 's2',
              'title': null,
              'updated_at': 'not-a-date',
              'created_at': '2026-09-17T09:00:00Z',
              'is_pinned': false,
            },
          ],
          'total': 2,
        }),
      ]);
      final api = newApi(http);
      final sessions = await api.listSessions();

      expect(sessions, hasLength(2));
      expect(sessions.first.id, 's1');
      expect(sessions.first.title, 'RAG 问答');
      expect(sessions.first.pinned, isTrue);
      expect(sessions.first.updatedAt, DateTime.utc(2026, 9, 19, 10));
      expect(sessions.first.createdAt, DateTime.utc(2026, 9, 18, 9));
      // Defensive defaults for missing/unparseable fields.
      expect(sessions.last.title, '');
      expect(sessions.last.pinned, isFalse);

      final request = http.requests.single;
      expect(request.path, 'api/v1/sessions');
      // The uri getter resolves the raw path against the Dio baseUrl, so
      // this locks the base-url join and the query parameter.
      expect(
        request.uri.toString(),
        'http://localhost:8084/api/v1/sessions?limit=50',
      );
      expect(request.queryParameters['limit'], 50);
      expect(request.headers['Authorization'], 'Bearer at-1');
    });

    test('passes a custom limit and tolerates a base URL without slash',
        () async {
      final http = _QueuedAdapter([
        _Reply.json({'data': <dynamic>[], 'total': 0}),
      ]);
      final api = newApi(http, baseUrl: 'http://x:8084');
      final sessions = await api.listSessions(limit: 7);
      expect(sessions, isEmpty);
      expect(
        http.requests.single.uri.toString(),
        'http://x:8084/api/v1/sessions?limit=7',
      );
    });
  });

  group('loadMessages', () {
    test('maps message fields, roles and reference lines', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': [
            {
              'id': 'm1',
              'session_id': 's1',
              'role': 'user',
              'content': '什么是 RAG?',
              'created_at': '2026-09-19T10:00:00Z',
            },
            {
              'id': 'm2',
              'session_id': 's1',
              'role': 'assistant',
              'content': 'RAG 是检索增强生成。',
              'created_at': '2026-09-19T10:00:01Z',
              'knowledge_references': [
                {
                  'source_title': 'Handbook',
                  'knowledge_snippet': 'RAG 检索文档片段',
                  'source_url': 'https://a/b',
                },
                {
                  'source_title': 'HTTPS 大写链接',
                  'knowledge_snippet': '大小写不敏感',
                  'source_url': 'HTTPS://A/B',
                },
                {
                  'source_title': '本地文档',
                  'knowledge_snippet': '非 http 链接',
                  'source_url': 'file:///tmp/doc',
                },
                'not-a-map',
                42,
                null,
                {
                  'source_title': 5,
                  'knowledge_snippet': null,
                  'source_url': 'https://c/d',
                },
                {
                  'source_title': '无链接',
                  'knowledge_snippet': '只有标题',
                },
              ],
            },
            {
              'id': 'm3',
              'session_id': 's1',
              'role': 'system',
              'content': 'preamble',
              'created_at': '2026-09-19T09:59:00Z',
            },
          ],
        }),
      ]);
      final api = newApi(http);
      final messages = await api.loadMessages(sessionId: 's1');

      final request = http.requests.single;
      expect(request.path, 'api/v1/messages/s1/load');
      expect(
        request.uri.toString(),
        'http://localhost:8084/api/v1/messages/s1/load?limit=200',
      );
      expect(request.queryParameters['limit'], 200);
      expect(request.headers['Authorization'], 'Bearer at-1');

      expect(messages, hasLength(3));
      expect(messages[0].role, 'user');
      expect(messages[0].content, '什么是 RAG?');
      expect(messages[0].sourceLines, isEmpty);

      expect(messages[1].role, 'assistant');
      expect(messages[1].content, 'RAG 是检索增强生成。');
      expect(messages[1].sourceLines, [
        '1. [Handbook](https://a/b) — RAG 检索文档片段',
        '2. [HTTPS 大写链接](HTTPS://A/B) — 大小写不敏感',
        '3. 本地文档 — 非 http 链接',
        // 'not-a-map', 42 and null entries are skipped; numbering stays dense.
        // Non-string fields are coerced so a malformed entry never crashes.
        '4. [5](https://c/d) — ',
        '5. 无链接 — 只有标题',
      ]);

      // System messages are kept with their role verbatim.
      expect(messages[2].role, 'system');
      expect(messages[2].content, 'preamble');
    });

    test('returns empty list when data is missing or malformed', () async {
      final http = _QueuedAdapter([
        _Reply.json({'success': true}),
      ]);
      final api = newApi(http);
      final messages = await api.loadMessages(sessionId: 's1');
      expect(messages, isEmpty);
    });

    test('404 throws WeKnoraSessionNotFoundException', () async {
      final http = _QueuedAdapter([
        _Reply.json({'error': 'not found'}, statusCode: 404),
      ]);
      final api = newApi(http);
      await expectLater(
        api.loadMessages(sessionId: 'gone'),
        throwsA(
          isA<WeKnoraSessionNotFoundException>().having(
            (e) => e.sessionId,
            'sessionId',
            'gone',
          ),
        ),
      );
      expect(http.requests.single.path, 'api/v1/messages/gone/load');
    });
  });
}

// Copied (behavior-identical) from
// test/features/weknora/weknora_auth_client_test.dart so this file stays
// self-contained.

Dio _dio(HttpClientAdapter adapter) {
  final dio = Dio();
  dio.httpClientAdapter = adapter;
  return dio;
}

final class _QueuedAdapter implements HttpClientAdapter {
  _QueuedAdapter(this._replies);

  final List<_Reply> _replies;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (_replies.isEmpty) throw StateError('No fake response remains.');
    return _replies.removeAt(0).toBody();
  }

  @override
  void close({bool force = false}) {}
}

final class _Reply {
  const _Reply(this.chunks, this.contentType, this.statusCode);

  factory _Reply.json(Map<String, dynamic> value, {int statusCode = 200}) =>
      _Reply(
        [utf8.encode(jsonEncode(value))],
        'application/json; charset=utf-8',
        statusCode,
      );

  final List<List<int>> chunks;
  final String contentType;
  final int statusCode;

  ResponseBody toBody() => ResponseBody(
    Stream<Uint8List>.fromIterable([
      for (final chunk in chunks) Uint8List.fromList(chunk),
    ]),
    statusCode,
    headers: {
      'content-type': [contentType],
    },
  );
}
