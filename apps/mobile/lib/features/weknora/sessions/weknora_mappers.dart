import 'package:conduit/core/models/chat_message.dart';
import 'package:conduit/core/models/conversation.dart';

import 'package:conduit/features/weknora/sessions/weknora_session_api.dart';

Conversation weknoraSessionToConversation(WeKnoraSessionSummary session) =>
    Conversation(
      id: 'weknora-${session.id}',
      title: session.title.isEmpty ? 'WeKnora' : session.title,
      createdAt: session.createdAt,
      updatedAt: session.updatedAt,
      pinned: session.pinned,
      metadata: {'weknoraSessionId': session.id},
    );

List<ChatMessage> weknoraHistoryToChatMessages(
  String sessionId,
  List<WeKnoraHistoryMessage> messages,
) {
  final chatMessages = <ChatMessage>[];
  for (final (index, message) in messages.indexed) {
    var content = message.content;
    if (message.role == 'assistant' && message.sourceLines.isNotEmpty) {
      content = '$content\n\n**Sources**\n${message.sourceLines.join('\n')}';
    }
    chatMessages.add(
      ChatMessage(
        id: 'weknora-$sessionId-$index',
        role: message.role,
        content: content,
        timestamp: DateTime.now(),
      ),
    );
  }
  return chatMessages;
}
