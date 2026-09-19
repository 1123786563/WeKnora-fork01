import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:conduit/core/models/chat_message.dart';
import 'package:conduit/core/models/conversation.dart';
import 'package:conduit/features/direct_connections/models/direct_completion.dart';
import 'package:conduit/features/direct_connections/models/direct_connection_profile.dart';
import 'package:conduit/features/direct_connections/services/weknora_adapter.dart';
import 'package:conduit/features/weknora/sessions/weknora_session_api.dart';
import 'package:conduit/features/weknora/sessions/weknora_session_sync.dart';

void main() {
  group('WeKnoraSessionSync.refreshSessions', () {
    test('upserts server sessions without touching local-only ones', () async {
      final localOnly = _conversation(
        'local-1',
        messages: [_message('m-local')],
      );
      final store = _ConversationStore()
        ..seed(
          _conversation(
            'weknora-s1',
            title: '旧标题',
            metadata: const {'weknoraSessionId': 's1'},
            messages: [_message('m-old')],
          ),
        )
        ..seed(localOnly);
      final api = _FakeSessionApi(
        sessions: [
          _session('s1', title: '新标题'),
          _session('s2', title: 'RAG 问答'),
        ],
      );
      final sync = store.buildSync(api);

      await sync.refreshSessions();

      // The stale server-backed row is refreshed in place.
      expect(store['weknora-s1']?.title, '新标题');
      expect(store['weknora-s1']?.metadata['weknoraSessionId'], 's1');
      // A brand-new server session is appended.
      expect(store['weknora-s2'], isNotNull);
      expect(store['weknora-s2']?.metadata['weknoraSessionId'], 's2');
      // The local-only conversation is never upserted and keeps its messages.
      expect(store['local-1'], same(localOnly));
      expect(store['local-1']?.messages, hasLength(1));
      expect(store.upsertedIds, ['weknora-s1', 'weknora-s2']);
      expect(api.listCalls, 1);
    });
  });

  group('WeKnoraSessionSync.hydrateMessages', () {
    test('replaces messages of the target conversation only', () async {
      final target = _conversation(
        'weknora-s1',
        metadata: const {'weknoraSessionId': 's1'},
        messages: [_message('m-stale')],
      );
      final other = _conversation(
        'weknora-s2',
        metadata: const {'weknoraSessionId': 's2'},
        messages: [_message('m-a'), _message('m-b')],
      );
      final store = _ConversationStore()
        ..seed(target)
        ..seed(other);
      final api = _FakeSessionApi(
        history: {
          's1': [
            const WeKnoraHistoryMessage(role: 'user', content: '什么是 RAG?'),
            const WeKnoraHistoryMessage(role: 'assistant', content: '检索增强生成。'),
            const WeKnoraHistoryMessage(
              role: 'assistant',
              content: '带来源的回答。',
              sourceLines: ['1. [Handbook](https://a/b) — 片段'],
            ),
          ],
        },
      );
      final sync = store.buildSync(api);

      await sync.hydrateMessages(conversation: target);

      expect(store.updatedIds, ['weknora-s1']);
      final hydrated = store['weknora-s1'];
      expect(hydrated?.messages, hasLength(3));
      expect(hydrated?.messages.first.id, 'weknora-s1-0');
      // Assistant messages fold their source lines into the content.
      expect(
        hydrated?.messages.last.content,
        '带来源的回答。\n\n**Sources**\n1. [Handbook](https://a/b) — 片段',
      );
      // Metadata survives the transform untouched.
      expect(hydrated?.metadata['weknoraSessionId'], 's1');
      // The other conversation is not reloaded by this call.
      expect(store['weknora-s2']?.messages, hasLength(2));
      expect(api.loadCalls, ['s1']);
    });

    test('evicts the conversation on 404', () async {
      final target = _conversation(
        'weknora-sX',
        metadata: const {'weknoraSessionId': 'sX'},
        messages: [_message('m-1')],
      );
      final store = _ConversationStore()..seed(target);
      final api = _FakeSessionApi(notFoundSessions: {'sX'});
      final sync = store.buildSync(api);

      await sync.hydrateMessages(conversation: target);

      expect(store.removedIds, ['weknora-sX']);
      expect(store['weknora-sX'], isNull);
      expect(store.updatedIds, isEmpty);
    });

    test('is a no-op when the conversation has no session metadata', () async {
      final local = _conversation('local-1');
      final store = _ConversationStore()..seed(local);
      final api = _FakeSessionApi(
        history: {
          's1': [const WeKnoraHistoryMessage(role: 'user', content: 'hi')],
        },
      );
      final sync = store.buildSync(api);

      await sync.hydrateMessages(conversation: local);

      expect(api.loadCalls, isEmpty);
      expect(store.updatedIds, isEmpty);
      expect(store.removedIds, isEmpty);
    });
  });

  group('WeKnoraSessionSync.stampBoundSessionId', () {
    test('writes merged metadata when the adapter knows the binding', () async {
      // WeKnoraAdapter is final, so the binding table is primed through the
      // public completion path: an explicit `weknora_session_id` parameter
      // registers a binding without any session-creation request.
      final http = _QueuedAdapter([
        _Reply.stream([
          utf8.encode(
            'event: message\n'
            'data: ${jsonEncode({'response_type': 'answer', 'content': 'hi'})}\n\n',
          ),
          utf8.encode(
            'event: message\n'
            'data: ${jsonEncode({'response_type': 'complete', 'done': true, 'usage': <String, dynamic>{}})}\n\n',
          ),
        ], contentType: 'text/event-stream'),
      ]);
      final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));
      final profile = _weknoraProfile();
      final run = adapter.startCompletion(
        profile,
        DirectCompletionRequest(
          remoteModelId: 'agent:ag1',
          messages: [DirectChatMessage.text(role: 'user', text: 'hello')],
          parameters: const {'weknora_session_id': 'sess-9'},
        ),
      );
      await run.events.toList();
      await run.done;

      final bound = _conversation(
        'c1',
        metadata: const {'otherKey': 'keep-me'},
        messages: [_message('m-1')],
      );
      final store = _ConversationStore()..seed(bound);
      final sync = store.buildSync(_FakeSessionApi());

      await sync.stampBoundSessionId(
        conversationId: 'c1',
        profileId: profile.id,
        remoteModelId: 'agent:ag1',
        priorUserQueries: const ['hello'],
        adapter: adapter,
      );

      expect(store.updatedIds, ['c1']);
      expect(store['c1']?.metadata['weknoraSessionId'], 'sess-9');
      // copyWith replaces the whole metadata map, so existing keys must be
      // merged back explicitly.
      expect(store['c1']?.metadata['otherKey'], 'keep-me');
    });

    test('does not update when the adapter has no binding', () async {
      final adapter = WeKnoraAdapter();
      final unbound = _conversation(
        'c2',
        metadata: const {'otherKey': 'value'},
      );
      final store = _ConversationStore()..seed(unbound);
      final sync = store.buildSync(_FakeSessionApi());

      await sync.stampBoundSessionId(
        conversationId: 'c2',
        profileId: 'weknora-1',
        remoteModelId: 'agent:ag1',
        priorUserQueries: const ['never-sent'],
        adapter: adapter,
      );

      expect(store.updatedIds, isEmpty);
      expect(store['c2']?.metadata.containsKey('weknoraSessionId'), isFalse);
    });
  });
}

Conversation _conversation(
  String id, {
  String title = 'Conversation',
  Map<String, dynamic> metadata = const {},
  List<ChatMessage> messages = const [],
}) {
  return Conversation(
    id: id,
    title: title,
    createdAt: DateTime.utc(2026, 9, 18),
    updatedAt: DateTime.utc(2026, 9, 19),
    messages: messages,
    metadata: metadata,
  );
}

ChatMessage _message(String id) => ChatMessage(
  id: id,
  role: 'user',
  content: 'content-$id',
  timestamp: DateTime.utc(2026, 9, 19),
);

WeKnoraSessionSummary _session(String id, {String title = 'Session'}) =>
    WeKnoraSessionSummary(
      id: id,
      title: title,
      updatedAt: DateTime.utc(2026, 9, 19, 10),
      createdAt: DateTime.utc(2026, 9, 18, 9),
      pinned: false,
    );

/// In-memory stand-in for the injected notifier seams.
class _ConversationStore {
  final Map<String, Conversation> _conversations = {};
  final List<String> upsertedIds = [];
  final List<String> updatedIds = [];
  final List<String> removedIds = [];

  void seed(Conversation conversation) =>
      _conversations[conversation.id] = conversation;

  Conversation? operator [](String id) => _conversations[id];

  void upsert(Conversation conversation) {
    upsertedIds.add(conversation.id);
    _conversations[conversation.id] = conversation;
  }

  void update(String id, Conversation Function(Conversation) transform) {
    final current = _conversations[id];
    if (current == null) return;
    updatedIds.add(id);
    _conversations[id] = transform(current);
  }

  void remove(String id) {
    removedIds.add(id);
    _conversations.remove(id);
  }

  WeKnoraSessionSync buildSync(WeKnoraSessionApi api) => WeKnoraSessionSync(
    api: api,
    upsertConversation: upsert,
    updateConversation: update,
    removeConversation: remove,
  );
}

class _FakeSessionApi implements WeKnoraSessionApi {
  _FakeSessionApi({
    this.sessions = const [],
    this.history = const {},
    this.notFoundSessions = const {},
  });

  final List<WeKnoraSessionSummary> sessions;
  final Map<String, List<WeKnoraHistoryMessage>> history;
  final Set<String> notFoundSessions;

  int listCalls = 0;
  final List<String> loadCalls = [];

  @override
  Future<List<WeKnoraSessionSummary>> listSessions({int limit = 50}) async {
    listCalls++;
    return sessions;
  }

  @override
  Future<List<WeKnoraHistoryMessage>> loadMessages({
    required String sessionId,
    int limit = 200,
  }) async {
    loadCalls.add(sessionId);
    if (notFoundSessions.contains(sessionId)) {
      throw WeKnoraSessionNotFoundException(sessionId);
    }
    return history[sessionId] ?? const [];
  }
}

// -- HTTP fakes (behavior-identical to the WeKnora adapter tests) so the
// stamp test can drive a real adapter through its public completion path.

DirectConnectionProfile _weknoraProfile() => DirectConnectionProfile(
  id: 'weknora-1',
  name: 'WeKnora',
  adapterKey: kWeKnoraAdapterKey,
  baseUrl: 'http://weknora.test:8080',
  apiKey: 'wk-key',
  apiKeyAuthMode: DirectApiKeyAuthMode.apiKeyHeader,
  manualModelIds: const [],
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

final class _Reply {
  const _Reply(this.chunks, this.contentType, this.statusCode);

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
