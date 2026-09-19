import 'package:conduit/features/direct_connections/models/direct_connection_profile.dart';

/// Extra Direct completion parameters for WeKnora conversations that already
/// know their server session. The parameter is WeKnora-only: other adapters
/// must never see it leak into their request bodies.
///
/// For non-WeKnora adapters this reproduces the send pipeline's legacy
/// expression exactly: `reasoning_effort` only for non-Ollama adapters, and an
/// empty map otherwise.
Map<String, dynamic> weknoraChatParameters({
  required String adapterKey,
  required Map<String, dynamic> conversationMetadata,
  String? reasoningEffort,
}) {
  final parameters = <String, dynamic>{};
  if (adapterKey != kOllamaAdapterKey && reasoningEffort != null) {
    parameters['reasoning_effort'] = reasoningEffort;
  }
  if (adapterKey == kWeKnoraAdapterKey) {
    final sessionId = conversationMetadata['weknoraSessionId'];
    if (sessionId is String && sessionId.isNotEmpty) {
      parameters['weknora_session_id'] = sessionId;
    }
  }
  return parameters;
}
