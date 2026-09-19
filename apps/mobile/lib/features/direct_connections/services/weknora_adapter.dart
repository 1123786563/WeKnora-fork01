import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:uuid/uuid.dart';

import '../../../core/services/sse_frame_scanner.dart';
import '../../../core/utils/debug_logger.dart';
import '../models/direct_completion.dart';
import '../models/direct_connection_profile.dart';
import '../models/direct_remote_model.dart';
import 'direct_adapter_helpers.dart';
import 'direct_http_client.dart';
import 'direct_provider_adapter.dart';

/// Direct adapter for a self-hosted WeKnora backend.
///
/// WeKnora (https://github.com/Tencent/WeKnora) is a session-oriented RAG
/// service rather than a raw completion API, so this adapter maps Conduit's
/// stateless completion contract onto WeKnora's server-side sessions:
///
/// - Model discovery exposes knowledge bases (`kb:<id>`, RAG chat), configured
///   agents (`agent:<id>`, agent chat), and chat models (`model:<id>`, direct
///   LLM chat). A bare manual id is treated as a chat model id.
/// - Each turn is matched to a WeKnora session by the sequence of prior user
///   queries, so normal conversations reuse one server session and only the
///   latest user message is sent. When the history no longer matches (edited
///   or regenerated turns), a new session is created and the prior turns are
///   replayed as a bounded context prefix.
/// - The `references` SSE frame maps to [DirectSourceFound] for web sources
///   and to a trailing markdown source list otherwise.
final class WeKnoraAdapter
    implements DirectProviderAdapter, CancellableDirectModelDiscovery {
  WeKnoraAdapter({
    DirectDioFactory? dioFactory,
    DirectHttpClientPool? clientPool,
    this.closeClients = true,
    this.streamIdleTimeout = kDirectStreamIdleTimeout,
    this.streamMaxDuration = kDirectStreamMaxDuration,
    this.maxStreamBytes = kMaxDirectStreamBytes,
    this.maxStreamCharacters = kMaxDirectStreamCharacters,
    this.maxStreamEvents = kMaxDirectStreamEvents,
    this.successDrainTimeout = kDirectSuccessDrainTimeout,
    this.maxSuccessDrainBytes = kMaxDirectSuccessDrainBytes,
  }) : _dioFactory = dioFactory,
       _clientPool = clientPool ?? DirectHttpClientPool(),
       _ownsClientPool = clientPool == null {
    validateDirectCompletionStreamLimits(
      idleTimeout: streamIdleTimeout,
      maxDuration: streamMaxDuration,
      maxBytes: maxStreamBytes,
      maxCharacters: maxStreamCharacters,
      maxEvents: maxStreamEvents,
    );
    if (successDrainTimeout <= Duration.zero) {
      throw ArgumentError.value(successDrainTimeout, 'successDrainTimeout');
    }
    if (maxSuccessDrainBytes <= 0) {
      throw RangeError.value(maxSuccessDrainBytes, 'maxSuccessDrainBytes');
    }
  }

  final DirectDioFactory? _dioFactory;
  final DirectHttpClientPool _clientPool;
  final bool _ownsClientPool;
  final bool closeClients;
  final Duration streamIdleTimeout;
  final Duration streamMaxDuration;
  final int maxStreamBytes;
  final int maxStreamCharacters;
  final int maxStreamEvents;
  final Duration successDrainTimeout;
  final int maxSuccessDrainBytes;

  static const int _maxSessionBindingsPerProfile = 32;
  static const int _maxImagesPerTurn = 8;
  static const int _maxContextPrefixCharacters = 6000;
  static const int _maxContextPerMessageCharacters = 1500;
  static const int _maxMarkdownSources = 20;
  static const int _maxMarkdownSourceCharacters = 4000;
  static const int _maxErrorBodyBytes = 64 * 1024;
  static const Duration _stopRequestTimeout = Duration(seconds: 5);

  /// Server-side sessions keyed by profile, oldest first.
  final Map<String, List<_WeKnoraSessionBinding>> _bindings = {};

  /// Tail of the currently running turn per WeKnora session id. WeKnora
  /// rejects concurrent turns in one agent session (HTTP 409) and interleaved
  /// history is undesirable even for normal chat, so turns serialize here.
  final Map<String, Future<void>> _turnTails = {};

  @override
  String get key => kWeKnoraAdapterKey;

  ({Dio dio, void Function() release}) _client(
    DirectConnectionProfile profile,
  ) {
    final factory = _dioFactory;
    if (factory != null) {
      final dio = factory(profile);
      const DirectHttpClientFactory().configure(dio, profile);
      return (
        dio: dio,
        release: () {
          if (closeClients) dio.close(force: true);
        },
      );
    }
    final lease = _clientPool.acquire(profile);
    return (dio: lease.dio, release: lease.release);
  }

  void dispose() {
    if (_ownsClientPool) _clientPool.dispose();
  }

  /// WeKnora accepts `X-API-Key` (workspace or platform key) or a JWT via
  /// `Authorization: Bearer`. The shared client factory only injects the
  /// Azure-style `api-key` header for [DirectApiKeyAuthMode.apiKeyHeader]
  /// profiles, so this adapter adds WeKnora's spelling itself. Bearer-mode
  /// profiles keep the factory's Authorization header (JWT login tokens).
  Map<String, String> _authHeaders(DirectConnectionProfile profile) {
    if (profile.apiKeyAuthMode == DirectApiKeyAuthMode.bearer) {
      return const <String, String>{};
    }
    final apiKey = profile.apiKey?.trim() ?? '';
    if (apiKey.isEmpty) return const <String, String>{};
    return <String, String>{'X-API-Key': apiKey};
  }

  /// WeKnora routes live under `/api/v1`. Accept profiles whose base URL
  /// already includes that prefix so either spelling works.
  String _apiPrefix(DirectConnectionProfile profile) {
    final base = profile.requestBaseUri().toString();
    return base.endsWith('api/v1/') ? '' : 'api/v1/';
  }

  @override
  Future<DirectConnectionProbe> probe(DirectConnectionProfile profile) async {
    final client = _client(profile);
    try {
      final response = await client.dio.get<ResponseBody>(
        '${_apiPrefix(profile)}knowledge-bases',
        options: Options(
          responseType: ResponseType.stream,
          headers: _authHeaders(profile),
        ),
      );
      final body = response.data;
      if (body == null) {
        throw const FormatException('WeKnora returned an empty body.');
      }
      final decoded = await decodeDirectJsonBody(body);
      final data = decoded['data'];
      return DirectConnectionProbe(
        reachable: true,
        modelCount: data is List ? data.length : null,
      );
    } catch (error) {
      final normalized = normalizeDirectProviderError(error);
      var message = sanitizeDirectProviderErrorMessage(
        normalized.message,
        sensitiveValues: directProfileSensitiveValues(profile),
      );
      if (normalized.statusCode == 401 || normalized.statusCode == 403) {
        message =
            '$message Send the workspace API key with the API key header '
            '(X-API-Key) or a JWT with Bearer authentication.';
      }
      return DirectConnectionProbe(reachable: false, message: message);
    } finally {
      client.release();
    }
  }

  @override
  Future<List<DirectRemoteModel>> listModels(
    DirectConnectionProfile profile,
  ) => _listModels(profile);

  @override
  Future<List<DirectRemoteModel>> listModelsCancellable(
    DirectConnectionProfile profile, {
    required DirectDiscoveryCancellation cancellation,
  }) => _listModels(profile, cancellation: cancellation);

  Future<List<DirectRemoteModel>> _listModels(
    DirectConnectionProfile profile, {
    DirectDiscoveryCancellation? cancellation,
  }) async {
    cancellation?.throwIfCancelled();
    final manualModels = directManualModels(profile);
    if (manualModels != null) return manualModels;

    final client = _client(profile);
    final dio = client.dio;
    final prefix = _apiPrefix(profile);
    final authHeaders = _authHeaders(profile);
    final requestCancellation = cancellation == null ? null : CancelToken();
    if (cancellation != null) {
      unawaited(
        cancellation.whenCancelled.then<void>((_) {
          if (!requestCancellation!.isCancelled) {
            requestCancellation.cancel('model discovery superseded');
          }
        }),
      );
    }

    Future<Object?> getJson(String path) async {
      final response = await dio.get<ResponseBody>(
        path,
        cancelToken: requestCancellation,
        options: Options(
          responseType: ResponseType.stream,
          headers: authHeaders,
        ),
      );
      final body = response.data;
      if (body == null) {
        throw const FormatException('WeKnora returned an empty body.');
      }
      return decodeDirectJsonValue(body);
    }

    try {
      // Required: doubles as the probe endpoint and needs only Viewer access.
      final knowledgeBases = await getJson('${prefix}knowledge-bases');
      cancellation?.throwIfCancelled();
      List<Object?>? chatModels;
      List<Object?>? agents;
      await Future.wait<void>([
        () async {
          try {
            final decoded = await getJson('${prefix}models');
            final data = decoded is Map ? decoded['data'] : null;
            if (data is List) chatModels = data;
            // A chat-scoped API key cannot list models (needs manage_models).
            // Discovery silently degrades to knowledge bases and agents.
          } on DirectDiscoveryCancelled {
            rethrow;
          } catch (_) {}
        }(),
        () async {
          try {
            final decoded = await getJson('${prefix}agents');
            final data = decoded is Map ? decoded['data'] : null;
            if (data is List) agents = data;
          } on DirectDiscoveryCancelled {
            rethrow;
          } catch (_) {}
        }(),
      ]);
      cancellation?.throwIfCancelled();

      final models = <DirectRemoteModel>[];
      final seen = <String>{};
      void addModel(DirectRemoteModel model) {
        if (model.id.isEmpty || !seen.add(model.id)) return;
        models.add(model);
      }

      // Knowledge bases first: they are the primary WeKnora chat surface.
      final kbData = knowledgeBases is Map ? knowledgeBases['data'] : null;
      if (kbData is List) {
        for (final item in kbData) {
          if (item is! Map) continue;
          final map = item.cast<String, dynamic>();
          final id = _trimmedString(map['id']);
          if (id == null) continue;
          final name = _trimmedString(map['name']) ?? id;
          final documentCount = map['knowledge_count'];
          final vlmConfig = map['vlm_config'];
          addModel(
            DirectRemoteModel(
              id: 'kb:$id',
              name: name,
              description: _knowledgeBaseDescription(map),
              isMultimodal: vlmConfig is Map && vlmConfig['enabled'] == true,
              capabilities: <String, dynamic>{
                'weknora_kind': 'knowledge_base',
                if (documentCount is int) 'documents': documentCount,
              },
            ),
          );
        }
      }
      if (agents != null) {
        for (final item in agents!) {
          if (item is! Map) continue;
          final map = item.cast<String, dynamic>();
          // Only smart-reasoning agents own the agent-chat pipeline; other
          // presets answer as plain knowledge chat and would 400 on agent_id.
          final config = map['config'];
          if (config is! Map ||
              config['agent_mode']?.toString() != 'smart-reasoning') {
            continue;
          }
          final id = _trimmedString(map['id']);
          if (id == null) continue;
          addModel(
            DirectRemoteModel(
              id: 'agent:$id',
              name: _trimmedString(map['name']) ?? id,
              description: _trimmedString(map['description']),
              capabilities: const <String, dynamic>{'weknora_kind': 'agent'},
            ),
          );
        }
      }
      if (chatModels != null) {
        for (final item in chatModels!) {
          if (item is! Map) continue;
          final map = item.cast<String, dynamic>();
          final type = map['type']?.toString();
          if (type != 'KnowledgeQA' && type != 'VLLM') continue;
          final id = _trimmedString(map['id']);
          if (id == null) continue;
          addModel(
            DirectRemoteModel(
              id: 'model:$id',
              name:
                  _trimmedString(map['display_name']) ??
                  _trimmedString(map['name']) ??
                  id,
              description: _trimmedString(map['description']),
              isMultimodal: type == 'VLLM',
              capabilities: const <String, dynamic>{'weknora_kind': 'model'},
            ),
          );
        }
      }
      cancellation?.throwIfCancelled();
      return List.unmodifiable(models);
    } on DirectDiscoveryCancelled {
      rethrow;
    } catch (error) {
      cancellation?.throwIfCancelled();
      final normalized = normalizeDirectProviderError(error);
      final safeMessage = sanitizeDirectProviderErrorMessage(
        normalized.message,
        sensitiveValues: directProfileSensitiveValues(profile),
      );
      DebugLogger.error(
        'models-failed',
        scope: 'direct-connections/weknora',
        error: safeMessage,
      );
      throw normalized;
    } finally {
      client.release();
    }
  }

  String? _knowledgeBaseDescription(Map<String, dynamic> map) {
    final description = _trimmedString(map['description']);
    final count = map['knowledge_count'];
    final countLabel = count is int ? '$count documents' : null;
    return [?description, ?countLabel].join(' · ');
  }

  @override
  DirectCompletionRun startCompletion(
    DirectConnectionProfile profile,
    DirectCompletionRequest request,
  ) {
    final client = _client(profile);
    final dio = client.dio;
    final cancelToken = CancelToken();
    final transportCancelToken = CancelToken();
    final controller = StreamController<DirectStreamEvent>();
    final settled = Completer<void>();
    final sensitiveValues = directProfileSensitiveValues(profile);
    var successfulProtocolTerminal = false;
    unawaited(
      cancelToken.whenCancel.then<void>((error) {
        if (!successfulProtocolTerminal && !transportCancelToken.isCancelled) {
          transportCancelToken.cancel(error.error ?? 'run cancelled');
        }
      }),
    );
    controller.onCancel = () {
      if (!successfulProtocolTerminal && !cancelToken.isCancelled) {
        cancelToken.cancel('listener cancelled');
      }
    };

    unawaited(
      Future<void>(() async {
        var terminalSent = false;
        void emitDone() {
          if (!terminalSent && !controller.isClosed) {
            successfulProtocolTerminal = true;
            terminalSent = true;
            controller.add(const DirectStreamDone());
          }
        }

        void emitSafeError(String message, {int? statusCode}) {
          if (!terminalSent && !controller.isClosed) {
            terminalSent = true;
            controller.add(DirectStreamError(message, statusCode: statusCode));
          }
        }

        void emitProtocolError(Object? payload, {int? statusCode}) {
          emitSafeError(
            directErrorMessage(payload, sensitiveValues: sensitiveValues),
            statusCode: statusCode,
          );
        }

        var transportCompletedCleanly = false;
        Completer<void>? turnTailOwned;
        _WeKnoraSessionBinding? binding;
        String? weknoraSessionId;
        String? assistantMessageId;
        try {
          rejectUnsupportedDirectToolParameters(request.parameters);
          if (request.enableImageGeneration || request.tools != null) {
            // WeKnora runs its own server-side agent tools; Conduit's local
            // tool runtime and image pipeline have no equivalent here.
            throw const DirectProviderException(
              'This WeKnora connection does not support that Direct capability.',
            );
          }
          final budget = DirectStreamBudget(
            maxCharacters: maxStreamCharacters,
            maxEvents: maxStreamEvents,
          );
          final messages = requireSerializableDirectMessages(request.messages);
          if (messages.any(
            (message) => message.parts.any((part) => part is DirectFilePart),
          )) {
            throw const DirectProviderException(
              'This WeKnora connection does not support file attachments.',
            );
          }
          final model = _WeKnoraModelRef.parse(request.remoteModelId);

          final priorQueries = <String>[];
          for (final message in messages) {
            if (message.role != 'user') continue;
            priorQueries.add(
              message.parts
                  .whereType<DirectTextPart>()
                  .map((part) => part.text)
                  .join('\n'),
            );
          }
          if (priorQueries.isEmpty) {
            throw const DirectProviderException(
              'The direct request has no user message to send.',
            );
          }
          final lastQuery = priorQueries.removeLast();
          final images = <String>[];
          for (final part
              in messages.lastWhere((message) => message.role == 'user').parts) {
            if (part is DirectImagePart &&
                part.url.startsWith('data:') &&
                images.length < _maxImagesPerTurn) {
              images.add(part.url);
            }
          }
          if (lastQuery.trim().isEmpty && images.isEmpty) {
            throw const DirectProviderException(
              'The direct request has no user message to send.',
            );
          }
          budget.add(lastQuery);

          final explicitSessionId = request.parameters['weknora_session_id'];
          binding = _matchBinding(profile.id, model, priorQueries);
          var queryToSend = lastQuery;
          if (explicitSessionId is String && explicitSessionId.isNotEmpty) {
            // Login-driven conversations know their server session; exact
            // binding replaces prefix guessing and never creates a
            // replacement session.
            weknoraSessionId = explicitSessionId;
            binding = _registerBinding(
              profile.id,
              model,
              explicitSessionId,
              priorQueries,
            );
          } else if (binding == null) {
            final sessionId = await _createSession(
              dio,
              profile,
              transportCancelToken,
            );
            weknoraSessionId = sessionId;
            binding = _registerBinding(
              profile.id,
              model,
              sessionId,
              priorQueries,
            );
            // The server session starts empty, so replay the local history as
            // a bounded context prefix when this conversation already has
            // turns (edited/regenerated/branched conversations).
            final contextPrefix = _contextPrefix(messages);
            if (contextPrefix != null) {
              queryToSend = '$contextPrefix\n\n---\n\n$queryToSend';
              budget.addCharacters(queryToSend.length);
            }
          } else {
            weknoraSessionId = binding.sessionId;
          }
          final activeBinding = binding;

          // Serialize turns per WeKnora session. WeKnora returns HTTP 409 for
          // concurrent agent turns and interleaved history corrupts normal
          // chat, so wait for any live turn in this session first.
          final previousTail =
              _turnTails[weknoraSessionId] ?? Future<void>.value();
          final tail = Completer<void>();
          _turnTails[weknoraSessionId] = tail.future;
          turnTailOwned = tail;
          await previousTail;
          if (cancelToken.isCancelled || controller.isClosed) {
            return;
          }

          final requestBody = <String, dynamic>{
            'query': queryToSend,
            'channel': 'api',
            if (model.kind == 'agent') ...<String, dynamic>{
              'agent_enabled': true,
              'agent_id': model.id,
            } else
              'disable_title': true,
            if (model.kind == 'kb') 'knowledge_base_ids': <String>[model.id],
            if (model.kind == 'model') 'summary_model_id': model.id,
            if (request.enableWebSearch) 'web_search_enabled': true,
            if (images.isNotEmpty) 'images': images,
          };

          final streamPath =
              '${_apiPrefix(profile)}'
              '${model.kind == 'agent' ? 'agent-chat' : 'knowledge-chat'}'
              '/$weknoraSessionId';
          Response<ResponseBody> response;
          try {
            response = await _postChatStream(
              dio,
              '$streamPath?resource_urls=public',
              requestBody,
              profile,
              transportCancelToken,
            );
          } on DioException catch (error) {
            // KB-restricted API keys may not request public resource URLs.
            if (error.response?.statusCode == 403) {
              response = await _postChatStream(
                dio,
                streamPath,
                requestBody,
                profile,
                transportCancelToken,
              );
            } else {
              rethrow;
            }
          }
          final body = response.data;
          if (body == null) {
            throw const FormatException('WeKnora returned an empty body.');
          }

          final references = <_WeKnoraReference>[];
          final linkedSourceUrls = <String>{};
          var hasCompletion = false;
          await for (final frame in _parseBoundedSse(
            directStreamingResponseBytes(
              body,
              idleTimeout: streamIdleTimeout,
              maxDuration: streamMaxDuration,
              maxBytes: maxStreamBytes,
              successfulProtocolTerminal: () => successfulProtocolTerminal,
              successDrainTimeout: successDrainTimeout,
              maxSuccessDrainBytes: maxSuccessDrainBytes,
            ),
          )) {
            if (controller.isClosed) break;
            if (terminalSent) {
              if (successfulProtocolTerminal) continue;
              break;
            }
            budget.addEvent();
            Object? payload;
            try {
              payload = jsonDecode(frame.data);
            } on FormatException {
              emitProtocolError(frame.data);
              break;
            }
            if (payload is! Map) {
              emitProtocolError(frame.data);
              break;
            }
            final frameMap = payload.cast<String, dynamic>();
            final responseType = frameMap['response_type']?.toString();
            final content = frameMap['content']?.toString();
            switch (responseType) {
              case 'answer':
                if (content != null && content.isNotEmpty) {
                  budget.add(content);
                  hasCompletion = hasCompletion || content.trim().isNotEmpty;
                  controller.add(DirectContentDelta(content));
                }
              case 'thinking':
                if (content != null && content.isNotEmpty) {
                  budget.add(content);
                  hasCompletion = hasCompletion || content.trim().isNotEmpty;
                  controller.add(DirectReasoningDelta(content));
                }
              case 'tool_call':
                final data = frameMap['data'];
                if (data is! Map) break;
                final line = _processLine(
                  prefix: 'Calling tool',
                  name: data['tool_name']?.toString(),
                  fallback: content,
                );
                if (line != null) {
                  budget.add(line);
                  controller.add(DirectReasoningDelta(line));
                }
              case 'tool_result':
                final data = frameMap['data'];
                if (data is! Map) break;
                // An absent `success` flag means the tool call succeeded.
                final line = data['success'] == false
                    ? _processLine(
                        prefix: 'Tool failed',
                        name: data['tool_name']?.toString(),
                        fallback: data['error']?.toString(),
                      )
                    : _processLine(
                        prefix: 'Tool finished',
                        name: data['tool_name']?.toString(),
                        fallback: null,
                      );
                if (line != null) {
                  budget.add(line);
                  controller.add(DirectReasoningDelta(line));
                }
              case 'references':
                final rawReferences = frameMap['knowledge_references'];
                if (rawReferences is List) {
                  for (final raw in rawReferences) {
                    if (raw is! Map) continue;
                    final reference = _WeKnoraReference.parse(
                      raw.cast<String, dynamic>(),
                    );
                    references.add(reference);
                    final url = reference.linkUrl;
                    if (url != null && linkedSourceUrls.add(url)) {
                      controller.add(
                        DirectSourceFound(
                          url: url,
                          title: reference.title,
                          snippet: reference.snippet,
                        ),
                      );
                    }
                  }
                }
              case 'agent_query':
                final data = frameMap['data'];
                assistantMessageId =
                    _trimmedString(frameMap['assistant_message_id']) ??
                    (data is Map
                        ? _trimmedString(data['assistant_message_id'])
                        : null);
              case 'complete':
                final markdown = _markdownSources(
                  references,
                  linkedSourceUrls,
                );
                if (markdown != null) {
                  budget.add(markdown);
                  controller.add(DirectContentDelta(markdown));
                }
                final usage = frameMap['usage'];
                if (usage is Map) {
                  try {
                    controller.add(
                      DirectUsageUpdate(
                        normalizeDirectUsageMetadata(
                          usage.cast<String, dynamic>(),
                        ),
                      ),
                    );
                  } on DirectProviderException {
                    // Malformed usage must not fail a completed answer.
                  }
                }
                hasCompletion = true;
                emitDone();
              case 'stop':
                emitDone();
              case 'error':
                emitProtocolError(
                  (content?.trim().isNotEmpty ?? false)
                      ? content
                      : frameMap['data'] ?? frameMap,
                );
              case 'session_title':
              case 'reflection':
              case 'memory_recalled':
              case 'user_message_injected':
              case 'context_compacted':
              case 'steer':
              case 'command_output':
              case 'install_output':
              case 'artifacts_pending':
              case 'tool_approval_required':
              case 'tool_approval_resolved':
              case 'mcp_oauth_required':
              case 'mcp_oauth_resolved':
              case 'install_prompt':
                break; // WeKnora lifecycle events; no Direct equivalent yet.
              default:
                break; // Unknown future frame types are skipped.
            }
          }
          if (!terminalSent && !cancelToken.isCancelled) {
            if (hasCompletion) {
              // The stream ended without WeKnora's complete marker (for
              // example an intermediary truncated it); keep the answer.
              emitDone();
            } else {
              throw const DirectProviderException(
                'The WeKnora stream ended before its completion marker.',
              );
            }
          }
          transportCompletedCleanly =
              transportCompletedCleanly || successfulProtocolTerminal;
          if (successfulProtocolTerminal) {
            activeBinding.recordCompletedQuery(lastQuery);
          }
        } catch (error) {
          final expectedDrainFailure =
              error is DirectStreamDrainException && successfulProtocolTerminal;
          if (!expectedDrainFailure &&
              !cancelToken.isCancelled &&
              !controller.isClosed) {
            final serverMessage = await _serverErrorMessage(error);
            final normalized = normalizeDirectProviderError(error);
            var safeMessage = sanitizeDirectProviderErrorMessage(
              serverMessage ?? normalized.message,
              sensitiveValues: sensitiveValues,
            );
            if (normalized.statusCode == 409) {
              safeMessage =
                  'WeKnora is already generating a reply in this chat session. '
                  'Wait for it to finish or stop it first.';
            }
            emitSafeError(safeMessage, statusCode: normalized.statusCode);
            DebugLogger.error(
              'completion-failed',
              scope: 'direct-connections/weknora',
              error: safeMessage,
            );
          }
        } finally {
          _completeTurnTail(weknoraSessionId, turnTailOwned);
          if (cancelToken.isCancelled &&
              weknoraSessionId != null &&
              assistantMessageId != null) {
            await _stopGeneration(
              dio,
              profile,
              weknoraSessionId,
              assistantMessageId,
            );
          }
          if (!transportCompletedCleanly && !transportCancelToken.isCancelled) {
            transportCancelToken.cancel('completion transport not reusable');
            // Dio observes cancellation through a future callback. Let that
            // callback abort the underlying request before `done` settles.
            await Future<void>.delayed(Duration.zero);
          }
          unawaited(controller.close());
          client.release();
          if (!settled.isCompleted) settled.complete();
        }
      }),
    );

    return DirectCompletionRun(
      id: const Uuid().v4(),
      profileId: profile.id,
      remoteModelId: request.remoteModelId,
      events: controller.stream,
      cancelToken: cancelToken,
      done: settled.future,
    );
  }

  Future<Response<ResponseBody>> _postChatStream(
    Dio dio,
    String path,
    Map<String, dynamic> requestBody,
    DirectConnectionProfile profile,
    CancelToken cancelToken,
  ) => dio.post<ResponseBody>(
    path,
    cancelToken: cancelToken,
    data: requestBody,
    options: Options(
      responseType: ResponseType.stream,
      receiveTimeout: streamIdleTimeout,
      headers: {..._authHeaders(profile), 'Accept': 'text/event-stream'},
    ),
  );

  Future<String> _createSession(
    Dio dio,
    DirectConnectionProfile profile,
    CancelToken cancelToken,
  ) async {
    final response = await dio.post<Map<String, dynamic>>(
      '${_apiPrefix(profile)}sessions',
      cancelToken: cancelToken,
      data: const <String, dynamic>{},
      options: Options(headers: _authHeaders(profile)),
    );
    final session = response.data?['data'];
    final id = session is Map ? _trimmedString(session['id']) : null;
    if (id == null) {
      throw const FormatException('WeKnora session creation failed.');
    }
    return id;
  }

  Future<void> _stopGeneration(
    Dio dio,
    DirectConnectionProfile profile,
    String sessionId,
    String messageId,
  ) async {
    try {
      await dio
          .post<void>(
            '${_apiPrefix(profile)}sessions/$sessionId/stop',
            data: <String, dynamic>{'message_id': messageId},
            options: Options(
              headers: _authHeaders(profile),
              receiveTimeout: _stopRequestTimeout,
              sendTimeout: _stopRequestTimeout,
            ),
          )
          .timeout(_stopRequestTimeout);
    } catch (_) {
      // Stopping is best-effort cleanup after a user cancellation.
    }
  }

  Stream<SseFrame> _parseBoundedSse(Stream<List<int>> bytes) async* {
    final scanner = SseFrameScanner();
    await for (final chunk in bytes.transform(utf8.decoder)) {
      for (final frame in scanner.addChunk(chunk)) {
        yield frame;
      }
    }
    for (final frame in scanner.close()) {
      yield frame;
    }
  }

  _WeKnoraSessionBinding? _matchBinding(
    String profileId,
    _WeKnoraModelRef model,
    List<String> priorQueries,
  ) {
    final bindings = _bindings[profileId];
    if (bindings == null) return null;
    for (final binding in bindings.reversed) {
      if (binding.remoteModelId != model.encoded) continue;
      if (binding.matches(priorQueries)) return binding;
    }
    return null;
  }

  /// Reads back the server session a conversation was bound to, keyed by the
  /// same prior-user-query sequence the binding table matches on. The login
  /// flow uses this to persist `weknoraSessionId` onto the conversation after
  /// a locally started turn.
  String? boundSessionIdFor({
    required String profileId,
    required String remoteModelId,
    required List<String> priorUserQueries,
  }) {
    final model = _WeKnoraModelRef.parse(remoteModelId);
    return _matchBinding(profileId, model, priorUserQueries)?.sessionId;
  }

  _WeKnoraSessionBinding _registerBinding(
    String profileId,
    _WeKnoraModelRef model,
    String sessionId,
    List<String> priorQueries,
  ) {
    final bindings = _bindings.putIfAbsent(profileId, () => <_WeKnoraSessionBinding>[]);
    if (bindings.length >= _maxSessionBindingsPerProfile) {
      bindings.removeRange(
        0,
        bindings.length - _maxSessionBindingsPerProfile + 1,
      );
    }
    final binding = _WeKnoraSessionBinding(
      remoteModelId: model.encoded,
      sessionId: sessionId,
      queries: List.of(priorQueries),
    );
    bindings.add(binding);
    return binding;
  }

  void _completeTurnTail(String? sessionId, Completer<void>? tail) {
    if (tail == null) return;
    if (!tail.isCompleted) tail.complete();
    if (sessionId != null && identical(_turnTails[sessionId], tail.future)) {
      _turnTails.remove(sessionId);
    }
  }

  /// Best-effort extraction of WeKnora's `{"code":..,"message":".."}` error
  /// body so HTTP failures surface the server's own message.
  Future<String?> _serverErrorMessage(Object error) async {
    if (error is! DioException) return null;
    final data = error.response?.data;
    try {
      Object? decoded;
      if (data is ResponseBody) {
        // Streaming requests leave error bodies unparsed; read once, bounded.
        decoded = await decodeDirectJsonValue(data, maxBytes: _maxErrorBodyBytes);
      } else if (data is Map) {
        decoded = data;
      }
      if (decoded is Map) {
        return _trimmedString(decoded['message']) ??
            _trimmedString(decoded['error']);
      }
    } catch (_) {
      // Error-body decoding is best-effort; fall back to the normalized error.
    }
    return null;
  }

  String? _processLine({
    required String prefix,
    required String? name,
    required String? fallback,
  }) {
    String? label = name?.trim();
    if (label == null || label.isEmpty) {
      final trimmed = fallback?.trim() ?? '';
      if (trimmed.isNotEmpty) label = trimmed;
    }
    if (label == null || label.isEmpty) return null;
    return '\n\n$prefix: $label\n';
  }

  /// Bounded transcript prefix for a fresh session that replays an existing
  /// local conversation (edited/regenerated/branched turns).
  String? _contextPrefix(List<DirectChatMessage> messages) {
    final lines = <String>[];
    var total = 0;
    DirectChatMessage? lastUserMessage;
    for (final message in messages) {
      if (message.role == 'user') lastUserMessage = message;
    }
    for (final message in messages) {
      if (identical(message, lastUserMessage)) break;
      final text = message.parts
          .whereType<DirectTextPart>()
          .map((part) => part.text)
          .join('\n')
          .trim();
      if (text.isEmpty) continue;
      final bounded = text.length > _maxContextPerMessageCharacters
          ? '${text.substring(0, _maxContextPerMessageCharacters)}…'
          : text;
      final line = '${message.role == 'user' ? 'User' : 'Assistant'}: $bounded';
      if (total + line.length > _maxContextPrefixCharacters) break;
      lines.add(line);
      total += line.length;
    }
    if (lines.isEmpty) return null;
    return '[Conversation context]\n${lines.join('\n\n')}\n\n[Current question]';
  }

  String? _markdownSources(
    List<_WeKnoraReference> references,
    Set<String> linkedSourceUrls,
  ) {
    final pending = references
        .where(
          (reference) =>
              reference.linkUrl == null ||
              !linkedSourceUrls.contains(reference.linkUrl),
        )
        .take(_maxMarkdownSources)
        .toList(growable: false);
    if (pending.isEmpty) return null;
    final buffer = StringBuffer('\n\n---\n**Sources**\n');
    var total = 0;
    for (var index = 0; index < pending.length; index++) {
      final reference = pending[index];
      final title = reference.title ?? 'Source ${index + 1}';
      final snippet = reference.snippet;
      final line = snippet == null
          ? '${index + 1}. $title'
          : '${index + 1}. $title — $snippet';
      if (total + line.length > _maxMarkdownSourceCharacters) break;
      buffer.writeln(line);
      total += line.length;
    }
    return buffer.toString();
  }
}

String? _trimmedString(Object? value) {
  if (value == null) return null;
  final text = value.toString().trim();
  return text.isEmpty ? null : text;
}

/// Parsed `remoteModelId` namespace: `kb:<id>`, `model:<id>`, `agent:<id>`.
/// A bare id defaults to a WeKnora chat model id.
final class _WeKnoraModelRef {
  const _WeKnoraModelRef(this.kind, this.id);

  factory _WeKnoraModelRef.parse(String raw) {
    final trimmed = raw.trim();
    final colon = trimmed.indexOf(':');
    if (colon > 0) {
      final kind = trimmed.substring(0, colon);
      final id = trimmed.substring(colon + 1).trim();
      if (id.isNotEmpty &&
          (kind == 'kb' || kind == 'model' || kind == 'agent')) {
        return _WeKnoraModelRef(kind, id);
      }
    }
    return _WeKnoraModelRef('model', trimmed);
  }

  final String kind;
  final String id;

  String get encoded => '$kind:$id';
}

/// One WeKnora server session bound to a Direct conversation prefix.
final class _WeKnoraSessionBinding {
  _WeKnoraSessionBinding({
    required this.remoteModelId,
    required this.sessionId,
    required List<String> queries,
  }) : queries = List.of(queries);

  final String remoteModelId;
  final String sessionId;

  /// User queries already sent to this server session, in order.
  List<String> queries;

  bool matches(List<String> priorQueries) {
    if (priorQueries.length != queries.length) return false;
    for (var index = 0; index < queries.length; index++) {
      if (queries[index] != priorQueries[index]) return false;
    }
    return true;
  }

  void recordCompletedQuery(String query) {
    if (queries.length >= _maxQueriesPerBinding) {
      queries.removeRange(
        0,
        queries.length - _maxQueriesPerBinding + 1,
      );
    }
    queries.add(query);
  }

  static const int _maxQueriesPerBinding = 200;
}

final class _WeKnoraReference {
  _WeKnoraReference._({
    required this.title,
    required this.linkUrl,
    required this.snippet,
  });

  static _WeKnoraReference parse(Map<String, dynamic> map) => _WeKnoraReference
      ._(
        title:
            _trimmedString(map['knowledge_title']) ??
            _trimmedString(map['knowledge_filename']),
        linkUrl: _httpSourceUrl(map['knowledge_source']),
        snippet: _boundedSnippet(map['content']),
      );

  final String? title;

  /// Absolute http(s) URL when the source itself is a web page; other chunks
  /// have no client-openable per-reference URL on the WeKnora API.
  final String? linkUrl;
  final String? snippet;

  static String? _httpSourceUrl(Object? value) {
    final source = _trimmedString(value);
    if (source == null) return null;
    final uri = Uri.tryParse(source);
    if (uri == null || !uri.hasScheme || uri.host.isEmpty) return null;
    if (uri.scheme != 'http' && uri.scheme != 'https') return null;
    return source;
  }

  static String? _boundedSnippet(Object? value) {
    final content = _trimmedString(value);
    if (content == null) return null;
    if (content.length <= _maxSourceSnippetCharacters) return content;
    return '${content.substring(0, _maxSourceSnippetCharacters)}…';
  }

  static const int _maxSourceSnippetCharacters = 300;
}
