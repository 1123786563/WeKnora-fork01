import 'package:flutter_test/flutter_test.dart';

import 'package:conduit/core/models/chat_message.dart';
import 'package:conduit/core/models/conversation.dart';
import 'package:conduit/features/weknora/sessions/weknora_mappers.dart';
import 'package:conduit/features/weknora/sessions/weknora_session_api.dart';

void main() {
  group('weknoraSessionToConversation', () {
    test('maps a session summary to a conversation with metadata binding', () {
      final conversation = weknoraSessionToConversation(
        WeKnoraSessionSummary(
          id: 's1',
          title: 'RAG 问答',
          updatedAt: DateTime.utc(2026, 9, 19),
          createdAt: DateTime.utc(2026, 9, 18),
          pinned: true,
        ),
      );
      expect(conversation.metadata['weknoraSessionId'], 's1');
      expect(conversation.id, 'weknora-s1');
      expect(conversation.title, 'RAG 问答');
      expect(conversation.pinned, isTrue);
    });

    test('falls back to a generic title for empty session titles', () {
      final conversation = weknoraSessionToConversation(
        WeKnoraSessionSummary(
          id: 's2',
          title: '',
          updatedAt: DateTime.utc(2026, 9, 19),
          createdAt: DateTime.utc(2026, 9, 18),
          pinned: false,
        ),
      );
      expect(conversation.title, 'WeKnora');
      expect(conversation.pinned, isFalse);
      expect(conversation.createdAt, DateTime.utc(2026, 9, 18));
      expect(conversation.updatedAt, DateTime.utc(2026, 9, 19));
      expect(conversation.messages, isEmpty);
    });
  });

  group('weknoraHistoryToChatMessages', () {
    test('maps history into user/assistant chat messages with source block',
        () {
      final messages = weknoraHistoryToChatMessages('s1', [
        WeKnoraHistoryMessage(
          role: 'user',
          content: '什么是 RAG?',
          sourceLines: const [],
        ),
        WeKnoraHistoryMessage(
          role: 'assistant',
          content: 'RAG 是检索增强生成。',
          sourceLines: const ['1. [Handbook](https://a/b) — 摘要片段'],
        ),
      ]);
      expect(messages.first.role, 'user');
      expect(messages.last.role, 'assistant');
      expect(messages.last.content, contains('Sources'));
      expect(messages.last.content, contains('https://a/b'));
    });

    test('binds each message id to the session and position', () {
      final messages = weknoraHistoryToChatMessages('s9', [
        WeKnoraHistoryMessage(role: 'user', content: 'a', sourceLines: const []),
        WeKnoraHistoryMessage(
            role: 'assistant', content: 'b', sourceLines: const []),
      ]);
      expect(messages.map((m) => m.id), ['weknora-s9-0', 'weknora-s9-1']);
    });

    test('keeps assistant content untouched when there are no sources', () {
      final messages = weknoraHistoryToChatMessages('s1', [
        WeKnoraHistoryMessage(
            role: 'assistant', content: 'Plain answer.', sourceLines: const []),
      ]);
      expect(messages.single.content, 'Plain answer.');
      expect(messages.single.content, isNot(contains('Sources')));
    });

    test('maps system messages as-is without a source block', () {
      final messages = weknoraHistoryToChatMessages('s1', [
        WeKnoraHistoryMessage(
          role: 'system',
          content: 'preamble',
          sourceLines: const ['1. [x](https://y/z) — s'],
        ),
      ]);
      expect(messages.single.role, 'system');
      expect(messages.single.content, 'preamble');
    });

    test('appends every source line to the assistant message', () {
      final messages = weknoraHistoryToChatMessages('s1', [
        WeKnoraHistoryMessage(
          role: 'assistant',
          content: 'Answer.',
          sourceLines: const [
            '1. [a](https://a/1) — sa',
            '2. b — sb',
          ],
        ),
      ]);
      expect(
        messages.single.content,
        'Answer.\n\n**Sources**\n1. [a](https://a/1) — sa\n2. b — sb',
      );
    });
  });

  group('ChatMessage shape', () {
    test('produced models satisfy core model invariants', () {
      final conversation = weknoraSessionToConversation(
        WeKnoraSessionSummary(
          id: 's1',
          title: 't',
          updatedAt: DateTime.utc(2026, 9, 19),
          createdAt: DateTime.utc(2026, 9, 18),
          pinned: false,
        ),
      );
      final messages = weknoraHistoryToChatMessages('s1', [
        WeKnoraHistoryMessage(
            role: 'user', content: 'q', sourceLines: const []),
      ]);
      expect(conversation, isA<Conversation>());
      expect(messages.single, isA<ChatMessage>());
      expect(messages.single.timestamp, isNotNull);
    });
  });
}
