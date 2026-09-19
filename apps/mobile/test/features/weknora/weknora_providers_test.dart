import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:conduit/core/models/chat_message.dart';
import 'package:conduit/core/models/conversation.dart';
import 'package:conduit/core/providers/app_providers.dart';
import 'package:conduit/features/weknora/account/weknora_account.dart';
import 'package:conduit/features/weknora/account/weknora_providers.dart';
import 'package:conduit/features/weknora/sessions/weknora_session_api.dart';
import 'package:conduit/features/weknora/sessions/weknora_session_sync.dart';

void main() {
  group('weknoraSessionRefreshAndHydrateProvider', () {
    test('hydrates every weknora conversation left empty after the refresh',
        () async {
      final api = _FakeSessionApi(
        sessions: [_session('s1'), _session('s2')],
        history: {
          's1': [
            const WeKnoraHistoryMessage(role: 'user', content: 'first turn'),
          ],
          's2': [
            const WeKnoraHistoryMessage(role: 'user', content: 'web turn'),
          ],
        },
      );
      final conversations = _FakeSyncConversations([
        // Previously hydrated: the refresh upsert blanks its transcript, so
        // it must be re-hydrated (the old behavior already did this).
        _conversation(
          'weknora-s1',
          metadata: const {'weknoraSessionId': 's1'},
          messages: [_message('m-old')],
        ),
        // Local-only: never touched.
        _conversation('local-1', messages: [_message('m-local')]),
      ]);
      final container = _container(api, conversations);
      addTearDown(container.dispose);

      await container.read(conversationsProvider.future);
      await container.read(weknoraSessionRefreshAndHydrateProvider)();

      // First-login case: s2 was upserted by the refresh with no messages
      // and still loads its history (the bug this provider fixes).
      expect(api.loadCalls, containsAll(['s1', 's2']));
      expect(conversations.items['weknora-s1']?.messages, hasLength(1));
      expect(conversations.items['weknora-s2']?.messages, hasLength(1));
      expect(conversations.items['local-1']?.messages, hasLength(1));
    });

    test('one conversation hydrate failure does not abort the rest',
        () async {
      final api = _FakeSessionApi(
        sessions: [_session('s1'), _session('s2')],
        history: {
          's2': [
            const WeKnoraHistoryMessage(role: 'user', content: 'ok'),
          ],
        },
        loadFailures: {'s1'},
      );
      final conversations = _FakeSyncConversations([
        _conversation(
          'weknora-s1',
          metadata: const {'weknoraSessionId': 's1'},
          messages: [_message('m-old')],
        ),
      ]);
      final container = _container(api, conversations);
      addTearDown(container.dispose);

      await container.read(conversationsProvider.future);
      // Must not throw despite s1's hydrate failing.
      await container.read(weknoraSessionRefreshAndHydrateProvider)();

      expect(api.loadCalls, containsAll(['s1', 's2']));
      expect(conversations.items['weknora-s2']?.messages, hasLength(1));
      // The failed conversation keeps its (blank) state; no eviction.
      expect(conversations.items['weknora-s1'], isNotNull);
    });

    test('reports conversations evicted because the session is gone (404)',
        () async {
      final api = _FakeSessionApi(
        sessions: [_session('s2')],
        history: {
          's2': [
            const WeKnoraHistoryMessage(role: 'user', content: 'ok'),
          ],
        },
        notFoundSessions: {'s1'},
      );
      final conversations = _FakeSyncConversations([
        _conversation(
          'weknora-s1',
          metadata: const {'weknoraSessionId': 's1'},
          messages: [_message('m-old')],
        ),
      ]);
      final container = _container(api, conversations);
      addTearDown(container.dispose);

      await container.read(conversationsProvider.future);
      final evicted =
          await container.read(weknoraSessionRefreshAndHydrateProvider)();

      expect(evicted, ['weknora-s1']);
      expect(conversations.items['weknora-s1'], isNull);
      expect(conversations.items['weknora-s2']?.messages, hasLength(1));
    });
  });

  group('weknoraSessionRefreshIfSignedInProvider', () {
    test('is a no-op without a signed-in account', () async {
      final api = _FakeSessionApi(sessions: [_session('s1')]);
      final conversations = _FakeSyncConversations(const []);
      final container = ProviderContainer(
        overrides: [
          conversationsProvider.overrideWith(() => conversations),
          weknoraSessionSyncProvider.overrideWithValue(
            WeKnoraSessionSync(
              api: api,
              upsertConversation: conversations.upsertConversation,
              updateConversation: conversations.updateConversation,
              removeConversation: conversations.removeConversation,
            ),
          ),
          weknoraAccountProvider.overrideWithValue(
            const AsyncValue<WeKnoraAccount?>.data(null),
          ),
        ],
      );
      addTearDown(container.dispose);

      final evicted =
          await container.read(weknoraSessionRefreshIfSignedInProvider)();

      expect(evicted, isEmpty);
      expect(api.listCalls, 0);
    });

    test('swallows refresh failures instead of throwing', () async {
      final api = _ThrowingListApi();
      final conversations = _FakeSyncConversations(const []);
      final container = ProviderContainer(
        overrides: [
          conversationsProvider.overrideWith(() => conversations),
          weknoraSessionSyncProvider.overrideWithValue(
            WeKnoraSessionSync(
              api: api,
              upsertConversation: conversations.upsertConversation,
              updateConversation: conversations.updateConversation,
              removeConversation: conversations.removeConversation,
            ),
          ),
          weknoraAccountProvider.overrideWithValue(
            AsyncValue<WeKnoraAccount?>.data(_account()),
          ),
        ],
      );
      addTearDown(container.dispose);

      final evicted =
          await container.read(weknoraSessionRefreshIfSignedInProvider)();

      expect(evicted, isEmpty);
    });
  });
}

WeKnoraAccount _account() => WeKnoraAccount(
  baseUrl: 'http://localhost:8084',
  email: 'a@b.c',
  userId: 'u1',
  displayName: 'alice',
  accessToken: 'at',
  refreshToken: 'rt',
);

class _ThrowingListApi extends _FakeSessionApi {
  @override
  Future<List<WeKnoraSessionSummary>> listSessions({int limit = 50}) async {
    throw const WeKnoraSessionApiException(
      statusCode: 500,
      message: 'boom',
    );
  }
}

ProviderContainer _container(
  _FakeSessionApi api,
  _FakeSyncConversations conversations,
) {
  return ProviderContainer(
    overrides: [
      conversationsProvider.overrideWith(() => conversations),
      weknoraSessionSyncProvider.overrideWithValue(
        WeKnoraSessionSync(
          api: api,
          upsertConversation: conversations.upsertConversation,
          updateConversation: conversations.updateConversation,
          removeConversation: conversations.removeConversation,
        ),
      ),
    ],
  );
}

Conversation _conversation(
  String id, {
  Map<String, dynamic> metadata = const {},
  List<ChatMessage> messages = const [],
}) {
  return Conversation(
    id: id,
    title: 'Conversation $id',
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

WeKnoraSessionSummary _session(String id) => WeKnoraSessionSummary(
  id: id,
  title: 'Session $id',
  updatedAt: DateTime.utc(2026, 9, 19, 10),
  createdAt: DateTime.utc(2026, 9, 18, 9),
  pinned: false,
);

/// In-memory conversations notifier: replaces the database-backed build and
/// the mutation methods the sync seams call.
class _FakeSyncConversations extends Conversations {
  _FakeSyncConversations(List<Conversation> items) : _items = items;

  final List<Conversation> _items;

  Map<String, Conversation> get items => {
    for (final conversation in _items) conversation.id: conversation,
  };

  @override
  Future<List<Conversation>> build() async => List.of(_items);

  @override
  void upsertConversation(
    Conversation conversation, {
    bool trustFolderConversation = false,
  }) {
    final index = _items.indexWhere((item) => item.id == conversation.id);
    if (index >= 0) {
      _items[index] = conversation;
    } else {
      _items.add(conversation);
    }
    state = AsyncData(List.of(_items));
  }

  @override
  void updateConversation(
    String id,
    Conversation Function(Conversation conversation) transform, {
    bool trustFolderConversation = false,
  }) {
    final index = _items.indexWhere((item) => item.id == id);
    if (index < 0) return;
    _items[index] = transform(_items[index]);
    state = AsyncData(List.of(_items));
  }

  @override
  void removeConversation(String id) {
    _items.removeWhere((item) => item.id == id);
    state = AsyncData(List.of(_items));
  }
}

class _FakeSessionApi implements WeKnoraSessionApi {
  _FakeSessionApi({
    this.sessions = const [],
    this.history = const {},
    this.notFoundSessions = const {},
    this.loadFailures = const {},
  });

  final List<WeKnoraSessionSummary> sessions;
  final Map<String, List<WeKnoraHistoryMessage>> history;
  final Set<String> notFoundSessions;
  final Set<String> loadFailures;

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
    if (loadFailures.contains(sessionId)) {
      throw const WeKnoraSessionApiException(
        statusCode: 500,
        message: 'boom',
      );
    }
    return history[sessionId] ?? const [];
  }

  @override
  Future<void> deleteSession({required String sessionId}) async {}
}
