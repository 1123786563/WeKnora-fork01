import 'package:flutter_test/flutter_test.dart';

import 'package:conduit/features/chat/services/weknora_chat_parameters.dart';
import 'package:conduit/features/direct_connections/models/direct_connection_profile.dart';

void main() {
  test('injects weknora_session_id only for weknora profiles with binding', () {
    final params = weknoraChatParameters(
      adapterKey: kWeKnoraAdapterKey,
      conversationMetadata: {'weknoraSessionId': 'sess-1'},
      reasoningEffort: null,
    );
    expect(params['weknora_session_id'], 'sess-1');
  });

  test('keeps empty map for other adapters even with metadata', () {
    expect(
      weknoraChatParameters(
        adapterKey: 'ollama',
        conversationMetadata: {'weknoraSessionId': 'sess-1'},
        reasoningEffort: null,
      ),
      isEmpty,
    );
  });

  test('preserves reasoning_effort alongside the session id', () {
    final params = weknoraChatParameters(
      adapterKey: kWeKnoraAdapterKey,
      conversationMetadata: {'weknoraSessionId': 'sess-1'},
      reasoningEffort: 'high',
    );
    expect(params['reasoning_effort'], 'high');
    expect(params['weknora_session_id'], 'sess-1');
  });

  test('matches the legacy expression for non-weknora adapters', () {
    // The send pipeline used to compute
    // `adapterKey == kOllamaAdapterKey || reasoningEffort == null
    //     ? {} : {'reasoning_effort': reasoningEffort}`.
    // Non-weknora profiles must keep byte-identical output.
    // Ollama never received reasoning_effort under the legacy expression.
    expect(
      weknoraChatParameters(
        adapterKey: kOllamaAdapterKey,
        conversationMetadata: const {},
        reasoningEffort: 'medium',
      ),
      isEmpty,
      reason: 'ollama must stay empty even with an effort configured',
    );
    const cases = {
      'openai-compatible': 'openai',
      'unknown': 'custom-adapter',
    };
    for (final entry in cases.entries) {
      expect(
        weknoraChatParameters(
          adapterKey: entry.value,
          conversationMetadata: const {},
          reasoningEffort: null,
        ),
        isEmpty,
        reason: '${entry.key} with null effort must stay empty',
      );
      expect(
        weknoraChatParameters(
          adapterKey: entry.value,
          conversationMetadata: const {},
          reasoningEffort: 'medium',
        ),
        const <String, dynamic>{'reasoning_effort': 'medium'},
        reason: '${entry.key} with effort must only carry reasoning_effort',
      );
    }
  });

  test('omits the session id when weknora metadata is missing or blank', () {
    expect(
      weknoraChatParameters(
        adapterKey: kWeKnoraAdapterKey,
        conversationMetadata: const {},
        reasoningEffort: null,
      ),
      isEmpty,
    );
    expect(
      weknoraChatParameters(
        adapterKey: kWeKnoraAdapterKey,
        conversationMetadata: const {'weknoraSessionId': ''},
        reasoningEffort: null,
      ),
      isEmpty,
    );
    expect(
      weknoraChatParameters(
        adapterKey: kWeKnoraAdapterKey,
        conversationMetadata: const {'weknoraSessionId': 42},
        reasoningEffort: null,
      ),
      isEmpty,
    );
  });
}
