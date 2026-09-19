import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:conduit/features/direct_connections/models/direct_completion.dart';
import 'package:conduit/features/direct_connections/models/direct_connection_profile.dart';
import 'package:conduit/features/direct_connections/services/weknora_adapter.dart';

void main() {
  group('WeKnoraAdapter.probe', () {
    test('reports reachability and knowledge-base count', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': [
            {'id': 'kb1'},
            {'id': 'kb2'},
          ],
        }),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final probe = await adapter.probe(_weknoraProfile());

      expect(probe.reachable, isTrue);
      expect(probe.modelCount, 2);
      expect(http.requests.single.path, 'api/v1/knowledge-bases');
      expect(
        http.requests.single.headers['X-API-Key'],
        'wk-key',
      );
    });

    test('surfaces an auth hint on 401', () async {
      final http = _QueuedAdapter([
        _Reply.json({'error': 'Unauthorized'}, statusCode: 401),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final probe = await adapter.probe(_weknoraProfile());

      expect(probe.reachable, isFalse);
      expect(probe.message, contains('X-API-Key'));
    });
  });

  group('WeKnoraAdapter.listModels', () {
    test('merges knowledge bases, agents, and chat models', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': [
            {
              'id': 'kb1',
              'name': 'Handbook',
              'description': 'Team docs',
              'knowledge_count': 12,
              'vlm_config': {'enabled': true},
            },
          ],
        }),
        _Reply.json({
          'success': true,
          'data': [
            {'id': 'm1', 'name': 'glm', 'type': 'KnowledgeQA'},
            {'id': 'm2', 'name': 'emb', 'type': 'Embedding'},
            {'id': 'm3', 'name': 'vlm', 'type': 'VLLM'},
          ],
        }),
        _Reply.json({
          'success': true,
          'data': [
            {
              'id': 'ag1',
              'name': 'Researcher',
              'config': {'agent_mode': 'smart-reasoning'},
            },
            {
              'id': 'ag2',
              'name': 'Quick',
              'config': {'agent_mode': 'quick-answer'},
            },
          ],
        }),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final models = await adapter.listModels(_weknoraProfile());

      expect(
        models.map((model) => model.id).toList(),
        ['kb:kb1', 'agent:ag1', 'model:m1', 'model:m3'],
      );
      expect(models.first.isMultimodal, isTrue);
      expect(models.first.description, contains('12 documents'));
      expect(models.last.isMultimodal, isTrue); // VLLM
    });

    test('degrades when the API key cannot list models or agents', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': [
            {'id': 'kb1', 'name': 'Handbook'},
          ],
        }),
        _Reply.json({'error': 'Forbidden'}, statusCode: 403),
        _Reply.json({'error': 'Forbidden'}, statusCode: 403),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final models = await adapter.listModels(_weknoraProfile());

      expect(models.map((model) => model.id), ['kb:kb1']);
    });

    test('manual model ids skip discovery network entirely', () async {
      final http = _QueuedAdapter(const <_Reply>[]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final models = await adapter.listModels(
        _weknoraProfile(manualModelIds: const ['kb:manual']),
      );

      expect(models.single.id, 'kb:manual');
      expect(http.requests, isEmpty);
    });
  });

  group('WeKnoraAdapter.startCompletion', () {
    test('knowledge-chat maps frames onto direct events', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': {'id': 's1'},
        }),
        _sse([
          {
            'id': 'q1',
            'response_type': 'agent_query',
            'done': true,
            'session_id': 's1',
            'assistant_message_id': 'am1',
          },
          {
            'response_type': 'references',
            'knowledge_references': [
              {
                'content': 'web snippet',
                'knowledge_title': 'Web source',
                'knowledge_source': 'https://example.com/page',
              },
              {
                'content': 'local chunk text',
                'knowledge_title': 'Handbook',
              },
            ],
          },
          {'response_type': 'answer', 'content': 'You'},
          {'response_type': 'answer', 'content': ' there'},
          {'response_type': 'session_title', 'content': 'New title'},
          {
            'response_type': 'complete',
            'done': true,
            'usage': {'prompt_tokens': 3, 'completion_tokens': 5},
          },
        ]),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final run = adapter.startCompletion(
        _weknoraProfile(),
        _request([
          DirectChatMessage.text(role: 'user', text: 'hello'),
        ], model: 'kb:kb1'),
      );
      final events = await run.events.toList();
      await run.done;

      expect(http.requests.length, 2);
      expect(http.requests.first.path, 'api/v1/sessions');
      final chatRequest = http.requests.last;
      expect(chatRequest.path, contains('knowledge-chat/s1'));
      expect(
        chatRequest.uri.queryParameters['resource_urls'],
        'public',
      );
      final body = chatRequest.data as Map<String, dynamic>;
      expect(body['query'], 'hello');
      expect(body['knowledge_base_ids'], ['kb1']);
      expect(body['disable_title'], isTrue);
      expect(body['channel'], 'api');

      final sources = events.whereType<DirectSourceFound>().toList();
      expect(sources.single.url, 'https://example.com/page');
      expect(sources.single.title, 'Web source');

      final content = events
          .whereType<DirectContentDelta>()
          .map((event) => event.content)
          .join();
      expect(content, startsWith('You there'));
      expect(content, contains('**Sources**'));
      expect(content, contains('Handbook'));

      expect(
        events.whereType<DirectUsageUpdate>().single.usage['prompt_tokens'],
        3,
      );
      expect(events.whereType<DirectStreamDone>(), isNotEmpty);
      expect(events.whereType<DirectStreamError>(), isEmpty);
    });

    test('follow-up turn reuses the server session and sends only the query',
        () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': {'id': 's1'},
        }),
        _sse([_completeFrame()]),
        // Second turn: no session creation reply queued.
        _sse([_completeFrame()]),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));
      final profile = _weknoraProfile();

      await _drain(
        adapter.startCompletion(
          profile,
          _request([
            DirectChatMessage.text(role: 'user', text: 'hello'),
          ], model: 'kb:kb1'),
        ),
      );
      await _drain(
        adapter.startCompletion(
          profile,
          _request([
            DirectChatMessage.text(role: 'user', text: 'hello'),
            DirectChatMessage.text(role: 'assistant', text: 'You there'),
            DirectChatMessage.text(role: 'user', text: 'more'),
          ], model: 'kb:kb1'),
        ),
      );

      expect(http.requests.length, 3);
      final followUp = http.requests.last;
      expect(followUp.path, contains('knowledge-chat/s1'));
      expect((followUp.data as Map<String, dynamic>)['query'], 'more');
    });

    test('edited history creates a new session with a context prefix',
        () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': {'id': 's1'},
        }),
        _sse([_completeFrame()]),
        // Second turn matches the binding and reuses s1.
        _sse([_completeFrame()]),
        _Reply.json({
          'success': true,
          'data': {'id': 's2'},
        }),
        _sse([_completeFrame()]),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));
      final profile = _weknoraProfile();

      await _drain(
        adapter.startCompletion(
          profile,
          _request([
            DirectChatMessage.text(role: 'user', text: 'hello'),
          ], model: 'kb:kb1'),
        ),
      );
      await _drain(
        adapter.startCompletion(
          profile,
          _request([
            DirectChatMessage.text(role: 'user', text: 'hello'),
            DirectChatMessage.text(role: 'assistant', text: 'You there'),
            DirectChatMessage.text(role: 'user', text: 'more'),
          ], model: 'kb:kb1'),
        ),
      );
      // The second user turn is now edited: the recorded binding
      // ['hello', 'more'] no longer matches the prior prefix ['hello'], so a
      // fresh session replays the conversation as context.
      await _drain(
        adapter.startCompletion(
          profile,
          _request([
            DirectChatMessage.text(role: 'user', text: 'hello'),
            DirectChatMessage.text(role: 'assistant', text: 'You there'),
            DirectChatMessage.text(role: 'user', text: 'edited'),
          ], model: 'kb:kb1'),
        ),
      );

      expect(http.requests.length, 5);
      final replayed = http.requests.last;
      expect(replayed.path, contains('knowledge-chat/s2'));
      final query = (replayed.data as Map<String, dynamic>)['query'] as String;
      expect(query, contains('[Conversation context]'));
      expect(query, contains('User: hello'));
      expect(query, contains('Assistant: You there'));
      expect(query.endsWith('edited'), isTrue);
    });

    test('agent-chat routes to agent-chat with agent fields', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': {'id': 's1'},
        }),
        _sse([
          {
            'response_type': 'agent_query',
            'done': true,
            'assistant_message_id': 'am1',
          },
          {'response_type': 'thinking', 'content': 'Let me check'},
          {
            'response_type': 'tool_call',
            'content': 'Calling tool: web_search',
            'data': {'tool_name': 'web_search'},
          },
          {
            'response_type': 'tool_result',
            'data': {'tool_name': 'web_search', 'success': true},
          },
          {'response_type': 'answer', 'content': 'Final'},
          {'response_type': 'complete', 'done': true},
        ]),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final events = await _drain(
        adapter.startCompletion(
          _weknoraProfile(),
          _request([
            DirectChatMessage.text(role: 'user', text: 'research this'),
          ], model: 'agent:ag1'),
        ),
      );

      final chatRequest = http.requests.last;
      expect(chatRequest.path, contains('agent-chat/s1'));
      final body = chatRequest.data as Map<String, dynamic>;
      expect(body['agent_id'], 'ag1');
      expect(body['agent_enabled'], isTrue);
      expect(body.containsKey('disable_title'), isFalse);

      final reasoning = events
          .whereType<DirectReasoningDelta>()
          .map((event) => event.content)
          .join();
      expect(reasoning, contains('Let me check'));
      expect(reasoning, contains('Calling tool: web_search'));
      expect(reasoning, contains('Tool finished: web_search'));
      expect(events.whereType<DirectStreamDone>(), isNotEmpty);
    });

    test('HTTP 409 surfaces a friendly message', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': {'id': 's1'},
        }),
        _Reply.json({
          'code': 1005,
          'message': 'another turn is already running in this session',
        }, statusCode: 409),
        // Retry-less path: the stream never started, so nothing more is read.
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final events = await _drain(
        adapter.startCompletion(
          _weknoraProfile(),
          _request([
            DirectChatMessage.text(role: 'user', text: 'hello'),
          ], model: 'agent:ag1'),
        ),
      );

      final error = events.whereType<DirectStreamError>().single;
      expect(error.statusCode, 409);
      expect(error.message, contains('already generating'));
    });

    test('in-stream error frames surface the server message', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': {'id': 's1'},
        }),
        _sse([
          {'response_type': 'answer', 'content': 'partial'},
          {
            'response_type': 'error',
            'content': 'No chat model available',
            'done': true,
          },
        ]),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final events = await _drain(
        adapter.startCompletion(
          _weknoraProfile(),
          _request([
            DirectChatMessage.text(role: 'user', text: 'hello'),
          ], model: 'kb:kb1'),
        ),
      );

      final error = events.whereType<DirectStreamError>().single;
      expect(error.message, contains('No chat model available'));
      expect(events.whereType<DirectStreamDone>(), isEmpty);
    });

    test('bearer profiles keep the shared Authorization header', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': {'id': 's1'},
        }),
        _sse([_completeFrame()]),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      await _drain(
        adapter.startCompletion(
          _weknoraProfile(
            authMode: DirectApiKeyAuthMode.bearer,
            apiKey: 'jwt-token',
          ),
          _request([
            DirectChatMessage.text(role: 'user', text: 'hello'),
          ], model: 'kb:kb1'),
        ),
      );

      final chatHeaders = http.requests.last.headers;
      expect(chatHeaders.containsKey('X-API-Key'), isFalse);
      expect(chatHeaders['Authorization'], 'Bearer jwt-token');
    });

    test('cancellation requests WeKnora to stop the in-flight message',
        () async {
      final openStream = _OpenEndAdapter([
        utf8.encode(
          'data: {"response_type":"agent_query","done":true,'
          '"assistant_message_id":"am1"}\n\n',
        ),
        utf8.encode(
          'data: {"response_type":"answer","content":"partial"}\n\n',
        ),
      ]);
      // Serves the session-creation reply once, then the open SSE stream.
      final mixed = _MixedAdapter(
        sessionReply: _Reply.json({
          'success': true,
          'data': {'id': 's1'},
        }),
        openStream: openStream,
      );
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(mixed));
      final run = adapter.startCompletion(
        _weknoraProfile(),
        _request([
          DirectChatMessage.text(role: 'user', text: 'hello'),
        ], model: 'agent:ag1'),
      );

      final events = <DirectStreamEvent>[];
      final sawContent = Completer<void>();
      late final StreamSubscription<DirectStreamEvent> subscription;
      subscription = run.events.listen((event) {
        events.add(event);
        if (event is DirectContentDelta && !sawContent.isCompleted) {
          sawContent.complete();
        }
      });
      await sawContent.future;
      await run.cancel();
      await subscription.cancel();

      final stopRequest = mixed.requests.last;
      expect(stopRequest.path, 'api/v1/sessions/s1/stop');
      expect(
        (stopRequest.data as Map<String, dynamic>)['message_id'],
        'am1',
      );
      expect(events.whereType<DirectStreamDone>(), isEmpty);
    });

    test('rejects requests without a user message before the network',
        () async {
      final http = _QueuedAdapter(const <_Reply>[]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final events = await _drain(
        adapter.startCompletion(
          _weknoraProfile(),
          _request([
            DirectChatMessage.text(role: 'assistant', text: 'only assistant'),
          ], model: 'kb:kb1'),
        ),
      );

      expect(http.requests, isEmpty);
      expect(
        events.whereType<DirectStreamError>().single.message,
        'The direct request has no user message to send.',
      );
    });

    test('resource_urls=public falls back once for restricted API keys',
        () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'data': {'id': 's1'},
        }),
        _Reply.json({'error': 'Forbidden'}, statusCode: 403),
        _sse([_completeFrame()]),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));

      final events = await _drain(
        adapter.startCompletion(
          _weknoraProfile(),
          _request([
            DirectChatMessage.text(role: 'user', text: 'hello'),
          ], model: 'kb:kb1'),
        ),
      );

      expect(events.whereType<DirectStreamDone>(), isNotEmpty);
      expect(
        http.requests.last.uri.queryParameters.containsKey('resource_urls'),
        isFalse,
      );
    });
  });
}

DirectCompletionRequest _request(
  List<DirectChatMessage> messages, {
  required String model,
}) => DirectCompletionRequest(remoteModelId: model, messages: messages);

Future<List<DirectStreamEvent>> _drain(DirectCompletionRun run) async {
  final events = await run.events.toList();
  await run.done;
  return events;
}

Map<String, dynamic> _completeFrame() => {
  'response_type': 'complete',
  'done': true,
  'usage': const <String, dynamic>{},
};

DirectConnectionProfile _weknoraProfile({
  String baseUrl = 'http://weknora.test:8080',
  String apiKey = 'wk-key',
  DirectApiKeyAuthMode authMode = DirectApiKeyAuthMode.apiKeyHeader,
  List<String> manualModelIds = const [],
}) => DirectConnectionProfile(
  id: 'weknora-1',
  name: 'WeKnora',
  adapterKey: kWeKnoraAdapterKey,
  baseUrl: baseUrl,
  apiKey: apiKey,
  apiKeyAuthMode: authMode,
  manualModelIds: manualModelIds,
);

_Reply _sse(List<Map<String, dynamic>> frames) => _Reply.stream(
  [
    for (final frame in frames)
      utf8.encode('event: message\ndata: ${jsonEncode(frame)}\n\n'),
  ],
  contentType: 'text/event-stream',
);

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

/// Serves the session-creation reply once, then an open-ended SSE stream.
final class _MixedAdapter implements HttpClientAdapter {
  _MixedAdapter({required this.sessionReply, required this.openStream});

  final _Reply sessionReply;
  final _OpenEndAdapter openStream;
  final List<RequestOptions> requests = [];
  bool _sessionCreated = false;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (!_sessionCreated) {
      _sessionCreated = true;
      return sessionReply.toBody();
    }
    return openStream.fetch(options, requestStream, cancelFuture);
  }

  @override
  void close({bool force = false}) {}
}

/// Emits fixed chunks and then keeps the SSE body open until cancellation.
final class _OpenEndAdapter implements HttpClientAdapter {
  _OpenEndAdapter(this.chunks);

  final List<List<int>> chunks;
  final Completer<void> cancelled = Completer<void>();

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (cancelFuture != null) {
      unawaited(
        cancelFuture.then((_) {
          if (!cancelled.isCompleted) cancelled.complete();
        }),
      );
    }
    late final StreamController<Uint8List> controller;
    controller = StreamController<Uint8List>(
      onListen: () {
        for (final chunk in chunks) {
          controller.add(Uint8List.fromList(chunk));
        }
      },
      onCancel: () {
        if (!cancelled.isCompleted) cancelled.complete();
      },
    );
    return ResponseBody(
      controller.stream,
      200,
      headers: {
        'content-type': ['text/event-stream'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

final class _Reply {
  const _Reply(this.chunks, this.contentType, this.statusCode);

  factory _Reply.json(
    Map<String, dynamic> value, {
    int statusCode = 200,
  }) => _Reply(
    [utf8.encode(jsonEncode(value))],
    'application/json; charset=utf-8',
    statusCode,
  );

  factory _Reply.stream(
    List<List<int>> chunks, {
    required String contentType,
    int statusCode = 200,
  }) => _Reply(chunks, contentType, statusCode);

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
