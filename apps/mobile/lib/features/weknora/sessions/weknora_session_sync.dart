import '../../../core/models/conversation.dart';
import '../../direct_connections/services/weknora_adapter.dart';
import 'weknora_mappers.dart';
import 'weknora_session_api.dart';

/// Bridges WeKnora's server-side session list into Conduit's conversation
/// list. All notifier mutations arrive as injected function seams so this
/// class stays pure Dart (no riverpod) and unit-testable with in-memory
/// fakes.
///
/// Conversation ids follow the mapper's `weknora-<sessionId>` convention and
/// carry `metadata['weknoraSessionId']`; conversations without that metadata
/// key are local-only and are never touched by this sync.
class WeKnoraSessionSync {
  WeKnoraSessionSync({
    required WeKnoraSessionApi api,
    required void Function(Conversation conversation) upsertConversation,
    required void Function(
      String id,
      Conversation Function(Conversation conversation) transform,
    )
    updateConversation,
    required void Function(String id) removeConversation,
  }) : _api = api,
       _upsertConversation = upsertConversation,
       _updateConversation = updateConversation,
       _removeConversation = removeConversation;

  final WeKnoraSessionApi _api;
  final void Function(Conversation conversation) _upsertConversation;
  final void Function(
    String id,
    Conversation Function(Conversation conversation) transform,
  )
  _updateConversation;
  final void Function(String id) _removeConversation;

  /// Pulls the first page of server sessions and upserts them. Local-only
  /// conversations (no weknoraSessionId metadata) are never touched.
  Future<void> refreshSessions() async {
    final sessions = await _api.listSessions();
    for (final session in sessions) {
      _upsertConversation(weknoraSessionToConversation(session));
    }
  }

  /// Loads server history into an existing conversation. A 404 (session
  /// deleted on the web) removes the conversation from the list.
  Future<void> hydrateMessages({required Conversation conversation}) async {
    final sessionId = conversation.metadata['weknoraSessionId'];
    if (sessionId is! String || sessionId.isEmpty) return;
    try {
      final history = await _api.loadMessages(sessionId: sessionId);
      final messages = weknoraHistoryToChatMessages(sessionId, history);
      _updateConversation(
        conversation.id,
        (current) =>
            current.copyWith(messages: messages, updatedAt: DateTime.now()),
      );
    } on WeKnoraSessionNotFoundException {
      _removeConversation(conversation.id);
    }
  }

  /// After a locally started turn completes, persist the server session the
  /// adapter bound this conversation to.
  Future<void> stampBoundSessionId({
    required String conversationId,
    required String profileId,
    required String remoteModelId,
    required List<String> priorUserQueries,
    required WeKnoraAdapter adapter,
  }) async {
    final sessionId = adapter.boundSessionIdFor(
      profileId: profileId,
      remoteModelId: remoteModelId,
      priorUserQueries: priorUserQueries,
    );
    if (sessionId == null) return;
    // copyWith replaces the metadata map wholesale, so spread the existing
    // entries first to keep unrelated keys.
    _updateConversation(
      conversationId,
      (current) => current.copyWith(
        metadata: {...current.metadata, 'weknoraSessionId': sessionId},
      ),
    );
  }
}
