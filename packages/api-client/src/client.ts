import { parseActionSuccessResponse, parseKnowledgeBaseListResponse, parseKnowledgeBaseResponse, type KnowledgeBase } from '@weknora/contracts';
import { ApiError, createAbortError, errorFromResult, isNamedError } from './errors.ts';
import type { HttpRequest, HttpResult, HttpTransport, NativeFileSource, UploadProgressEvent } from './ports.ts';
import { createKnowledgeDocumentsApi } from './knowledge/documents.ts';
import { createKnowledgeFaqApi } from './knowledge/faq.ts';
import { createWikiPagesApi } from './wiki/pages.ts';
import { createCommercialApi } from './commercial.ts';
import { createAppConnectorApi } from './appconnector.ts';
import { createDataSourcesApi } from './datasource.ts';
import { createAuthApi } from './auth/endpoints.ts';
import { createChatSessionsApi } from './chat/sessions.ts';
import { createSandboxTerminalApi } from './sandbox/terminal.ts';
import { createSandboxSkillInstallApi } from './sandbox/skill-install.ts';
import { createSandboxConfigurationsApi } from './sandbox-configurations.ts';
import { createConfigurationApi } from './configuration.ts';
import { buildChatStreamRequest, consumeChatStream, consumeStreamResult, createServerSentEventParser, parseChatEvent } from './chat/stream.ts';
import { createChatApprovalsApi } from './chat/approvals.ts';
import { createChatSteerApi } from './chat/steer.ts';
import { createChatAttachmentsApi } from './chat/attachments.ts';
import { createChatSuggestionsApi } from './chat/suggestions.ts';
import { createChatArtifactsApi } from './chat/artifacts.ts';
import { createIdentityApi } from './identity/index.ts';
import { createAdministrationApi } from './administration/index.ts';
import { createSettingsApi } from './settings/index.ts';
import { createEmbedApi } from './embed/index.ts';
import { createKnowledgeSettingsApi } from './knowledge/settings.ts';

export type { KnowledgeBase } from '@weknora/contracts';

export interface ClientRequest {
  method: string;
  path: string;
  headers?: Record<string, string>;
  body?: unknown;
  nativeFile?: NativeFileSource;
  multipartFields?: Record<string, string>;
  onProgress?: (progress: UploadProgressEvent) => void;
  signal?: AbortSignal;
}

export interface ClientBinaryResponse {
  body: string | Blob | ArrayBuffer;
  contentType?: string;
  headers: Record<string, string>;
}

export interface WeKnoraClientOptions {
  baseURL: string;
  transport: HttpTransport;
  timeoutMs?: number;
}

export interface KnowledgeBaseListParams {
  agent_id?: string;
  agent_source_tenant_id?: string;
  creator?: 'all' | 'mine' | 'others';
}

export interface KnowledgeBaseMutationInput {
  name: string;
  description?: string;
  type?: 'document' | 'faq';
  [key: string]: unknown;
}

function joinURL(baseURL: string, path: string): string {
  const base = baseURL.replace(/\/+$/, '');
  const suffix = path.startsWith('/') ? path : `/${path}`;
  return `${base}${suffix}`;
}

function multipartBody(fields: Record<string, string>): FormData {
  if (typeof FormData === 'undefined') throw new Error('multipart form data is unavailable');
  const form = new FormData();
  for (const [key, value] of Object.entries(fields)) form.append(key, value);
  return form;
}

function withQuery(path: string, params: Record<string, string | undefined>): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) if (value !== undefined) query.set(key, value);
  const suffix = query.toString();
  return suffix ? `${path}?${suffix}` : path;
}

/** Pin/duplicate responses are success envelopes whose payload is not a full KnowledgeBase. */
function parseKnowledgeBaseActionData(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object') throw new Error('knowledge-base action response must be an object');
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) throw new Error('knowledge-base action failed');
  const data = envelope.data;
  if (data === null || typeof data !== 'object') throw new Error('knowledge-base action response.data must be an object');
  return data as Record<string, unknown>;
}

/**
 * POST /api/v1/knowledge-search (internal/handler/session/qa.go SearchKnowledge,
 * Viewer+; routed at internal/router/routes_chat.go:127). Semantic/keyword
 * chunk search across one or more knowledge bases, without LLM summarization —
 * the backend the Vue GlobalCommandPalette's useCmdkSearch() chunk group calls
 * via knowledgeSemanticSearch() (frontend/src/api/knowledge-base/index.ts:634).
 * The backend requires at least one of knowledge_base_ids/knowledge_ids/a tag
 * scope (qa.go:826-830) — callers must not invoke this with an empty scope.
 */
export interface KnowledgeChunkSearchParams {
  query: string;
  knowledgeBaseIds: readonly string[];
  knowledgeIds?: readonly string[];
  signal?: AbortSignal;
}

export interface KnowledgeChunkSearchHit {
  id: string;
  content: string;
  matchedContent: string;
  knowledgeId: string;
  knowledgeBaseId: string;
  knowledgeTitle: string;
  knowledgeFilename: string;
  chunkIndex: number;
  score: number;
  matchType: string;
}

function parseKnowledgeChunkSearchHit(value: unknown, index: number): KnowledgeChunkSearchHit {
  const path = `/knowledge-search.data[${index}]`;
  if (value === null || typeof value !== 'object') throw new Error(`${path} must be an object`);
  const row = value as Record<string, unknown>;
  if (typeof row.id !== 'string') throw new Error(`${path}.id must be a string`);
  if (typeof row.knowledge_id !== 'string') throw new Error(`${path}.knowledge_id must be a string`);
  return {
    id: row.id,
    content: typeof row.content === 'string' ? row.content : '',
    matchedContent: typeof row.matched_content === 'string' ? row.matched_content : '',
    knowledgeId: row.knowledge_id,
    knowledgeBaseId: typeof row.knowledge_base_id === 'string' ? row.knowledge_base_id : '',
    knowledgeTitle: typeof row.knowledge_title === 'string' ? row.knowledge_title : '',
    knowledgeFilename: typeof row.knowledge_filename === 'string' ? row.knowledge_filename : '',
    chunkIndex: typeof row.chunk_index === 'number' ? row.chunk_index : 0,
    score: typeof row.score === 'number' ? row.score : 0,
    matchType: typeof row.match_type === 'string' ? row.match_type : '',
  };
}

function parseKnowledgeChunkSearchResponse(value: unknown): KnowledgeChunkSearchHit[] {
  if (value === null || typeof value !== 'object') throw new Error('/knowledge-search response must be an object');
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) throw new Error('/knowledge-search request failed');
  const data = envelope.data;
  if (!Array.isArray(data)) throw new Error('/knowledge-search.data must be an array');
  return data.map((item, index) => parseKnowledgeChunkSearchHit(item, index));
}

export function createWeKnoraClient(options: WeKnoraClientOptions) {
  const timeoutMs = options.timeoutMs ?? 30_000;

  async function request(input: ClientRequest): Promise<unknown> {
    const controller = new AbortController();
    let timedOut = false;
    const cancel = () => controller.abort();
    if (input.signal?.aborted) controller.abort();
    else input.signal?.addEventListener('abort', cancel, { once: true });
    const timer = setTimeout(() => { timedOut = true; controller.abort(); }, timeoutMs);
    const request: HttpRequest = {
      method: input.method,
      url: joinURL(options.baseURL, input.path),
      headers: { accept: 'application/json', ...input.headers },
      body: input.body ?? (input.multipartFields === undefined ? undefined : multipartBody(input.multipartFields)),
      signal: controller.signal,
    };
    try {
      const result: HttpResult = input.nativeFile && options.transport.sendMultipartFile
        ? await options.transport.sendMultipartFile({
          method: request.method,
          url: request.url,
          headers: request.headers,
          file: input.nativeFile,
          fields: input.multipartFields ?? {},
          signal: request.signal,
          ...(input.onProgress ? { onProgress: input.onProgress } : {}),
        })
        : await options.transport.send(request);
      if (controller.signal.aborted) {
        throw createAbortError();
      }
      if (result.status === 204) return undefined;
      if (result.status < 200 || result.status >= 300) throw errorFromResult(result.status, result.body, result.headers);
      return result.body;
    } catch (error: unknown) {
      if (error instanceof ApiError) throw error;
      if (timedOut || isNamedError(error, 'TimeoutError')) {
        throw new ApiError({ code: 'TIMEOUT', message: 'Request timed out', cause: error });
      }
      if (input.signal?.aborted || isNamedError(error, 'AbortError')) {
        throw new ApiError({ code: 'CANCELLED', message: 'Request was cancelled', cause: error });
      }
      throw error;
    } finally {
      clearTimeout(timer);
      input.signal?.removeEventListener('abort', cancel);
    }
  }

  async function requestBinary(input: ClientRequest): Promise<ClientBinaryResponse> {
    if (!options.transport.sendBinary) throw new Error('Binary transport is unavailable');
    const controller = new AbortController();
    let timedOut = false;
    const cancel = () => controller.abort();
    if (input.signal?.aborted) controller.abort();
    else input.signal?.addEventListener('abort', cancel, { once: true });
    const timer = setTimeout(() => { timedOut = true; controller.abort(); }, timeoutMs);
    const binaryRequest: HttpRequest = {
      method: input.method,
      url: joinURL(options.baseURL, input.path),
      headers: { accept: '*/*', ...input.headers },
      body: input.body ?? (input.multipartFields === undefined ? undefined : multipartBody(input.multipartFields)),
      signal: controller.signal,
    };
    try {
      const result = await options.transport.sendBinary(binaryRequest);
      if (controller.signal.aborted) throw createAbortError();
      if (result.status < 200 || result.status >= 300) throw errorFromResult(result.status, result.body, result.headers);
      const body = result.body;
      if (typeof body !== 'string' && !(typeof Blob !== 'undefined' && body instanceof Blob) && !(body instanceof ArrayBuffer)) {
        throw new Error('Binary request returned an unsupported body');
      }
      return { body, contentType: result.headers['content-type'], headers: result.headers };
    } catch (error: unknown) {
      if (error instanceof ApiError) throw error;
      if (timedOut || isNamedError(error, 'TimeoutError')) throw new ApiError({ code: 'TIMEOUT', message: 'Request timed out', cause: error });
      if (input.signal?.aborted || isNamedError(error, 'AbortError')) throw new ApiError({ code: 'CANCELLED', message: 'Request was cancelled', cause: error });
      throw error;
    } finally {
      clearTimeout(timer);
      input.signal?.removeEventListener('abort', cancel);
    }
  }

  const knowledgeDocuments = createKnowledgeDocumentsApi(request, requestBinary);
  const knowledgeFaq = createKnowledgeFaqApi(request);
  const knowledgeSettings = createKnowledgeSettingsApi(request);
  const wiki = createWikiPagesApi(request);
  const dataSources = createDataSourcesApi(request);
  const commercial = createCommercialApi(request);
  const apps = createAppConnectorApi(request);
  const auth = createAuthApi(request);
  const sessions = createChatSessionsApi(request);
  const sandbox = createSandboxTerminalApi(request);
  // Skill install streams ride the same transport chain (sendStream when the
  // platform provides one, buffered request otherwise) so auth/tenant headers
  // and refresh behave exactly like every other call.
  const sandboxSkills = createSandboxSkillInstallApi({
    request,
    ...(options.transport.sendStream
      ? {
        sendStream: (input: ClientRequest) => options.transport.sendStream!({
          method: input.method,
          url: joinURL(options.baseURL, input.path),
          headers: { accept: 'text/event-stream', ...input.headers },
          body: input.body,
          ...(input.signal === undefined ? {} : { signal: input.signal }),
        }),
      }
      : {}),
  });
  const sandboxConfigurations = createSandboxConfigurationsApi(request);
  const configuration = createConfigurationApi(request);
  const chatApprovals = createChatApprovalsApi(request);
  const chatSteer = createChatSteerApi(request);
  const chatAttachments = createChatAttachmentsApi(request);
  const chatSuggestions = createChatSuggestionsApi(request);
  const chatArtifacts = createChatArtifactsApi(request, requestBinary);
  const identity = createIdentityApi(request);
  const administration = createAdministrationApi(request);
  const settings = createSettingsApi(request);
  const embed = createEmbedApi(request, async (streamRequest, onEvent, signal) => {
    const input = signal === undefined ? streamRequest : { ...streamRequest, signal };
    if (options.transport.sendStream) {
      const result = await options.transport.sendStream({
        method: input.method,
        url: joinURL(options.baseURL, input.path),
        headers: { accept: 'application/json', ...input.headers },
        body: input.body,
        signal: input.signal,
      });
      await consumeStreamResult(result, onEvent);
      return;
    }
    const body = await request(input);
    if (typeof body !== 'string') throw new Error('Embed chat stream returned a non-text body');
    const parser = createServerSentEventParser((event) => onEvent(parseChatEvent(event)));
    parser.push(body);
    parser.finish();
  });

  async function consumeChatTransport(input: ClientRequest, onEvent: Parameters<typeof consumeChatStream>[2]): Promise<void> {
    if (!options.transport.sendStream) {
      const body = await request(input);
      if (typeof body !== 'string') throw new Error('Chat stream returned a non-text body');
      const parser = createServerSentEventParser((event) => onEvent(parseChatEvent(event)));
      parser.push(body);
      parser.finish();
      return;
    }
    const result = await options.transport.sendStream({
      method: input.method,
      url: joinURL(options.baseURL, input.path),
      headers: { accept: 'text/event-stream', ...input.headers },
      body: input.body,
      signal: input.signal,
    });
    await consumeStreamResult(result, onEvent);
  }

  return {
    request,
    requestBinary,
    knowledgeBases: {
      async list(params: KnowledgeBaseListParams = {}): Promise<KnowledgeBase[]> {
        const path = withQuery('/api/v1/knowledge-bases', { ...params });
        return parseKnowledgeBaseListResponse(await request({ method: 'GET', path }));
      },
      async create(input: KnowledgeBaseMutationInput): Promise<KnowledgeBase> {
        return parseKnowledgeBaseResponse(await request({ method: 'POST', path: '/api/v1/knowledge-bases', body: input }));
      },
      async update(id: string, input: KnowledgeBaseMutationInput): Promise<KnowledgeBase> {
        return parseKnowledgeBaseResponse(await request({ method: 'PUT', path: `/api/v1/knowledge-bases/${encodeURIComponent(id)}`, body: input }));
      },
      /** POST /api/v1/knowledge/batch-reparse (routes_knowledge.go:129, Contributor+;
 *  payload {kb_id, ids} per internal/handler/knowledge.go:2580-2584). */
      async batchReparse(knowledgeBaseId: string, documentIds: readonly string[]): Promise<void> {
        if (documentIds.length === 0) return;
        await request({
          method: 'POST',
          path: '/api/v1/knowledge/batch-reparse',
          body: { kb_id: knowledgeBaseId, ids: [...documentIds] },
        });
      },
      async remove(id: string): Promise<void> {
        await request({ method: 'DELETE', path: `/api/v1/knowledge-bases/${encodeURIComponent(id)}` });
      },
      async search(params: KnowledgeChunkSearchParams): Promise<KnowledgeChunkSearchHit[]> {
        if (params.query.trim() === '') throw new Error('query must not be empty');
        if (params.knowledgeBaseIds.length === 0 && (params.knowledgeIds ?? []).length === 0) {
          throw new Error('search requires at least one knowledgeBaseId or knowledgeId');
        }
        const body: Record<string, unknown> = { query: params.query, knowledge_base_ids: [...params.knowledgeBaseIds] };
        if (params.knowledgeIds?.length) body.knowledge_ids = [...params.knowledgeIds];
        return parseKnowledgeChunkSearchResponse(await request({
          method: 'POST', path: '/api/v1/knowledge-search', body, ...(params.signal === undefined ? {} : { signal: params.signal }),
        }));
      },
      async togglePin(id: string): Promise<{ is_pinned: boolean }> {
        const data = parseKnowledgeBaseActionData(await request({ method: 'PUT', path: `/api/v1/knowledge-bases/${encodeURIComponent(id)}/pin` }));
        return { is_pinned: data.is_pinned === true };
      },
      async duplicate(id: string): Promise<{ target_id: string }> {
        const data = parseKnowledgeBaseActionData(await request({ method: 'POST', path: `/api/v1/knowledge-bases/${encodeURIComponent(id)}/duplicate` }));
        const targetId = typeof data.target_id === 'string' && data.target_id ? data.target_id : (typeof data.id === 'string' ? data.id : '');
        if (!targetId) throw new Error('knowledge-bases/duplicate.data.target_id must be a non-empty string');
        return { target_id: targetId };
      },
      documents: knowledgeDocuments,
      faq: knowledgeFaq,
      settings: knowledgeSettings,
    },
    knowledge: {
      documents: knowledgeDocuments,
      faq: knowledgeFaq,
      settings: knowledgeSettings,
    },
    wiki,
    dataSources,
    commercial,
    apps,
    auth,
    identity,
    administration,
    settings,
    embed,
    sessions,
    sandbox: { issueTicket: sandbox.issueTicket, skills: sandboxSkills },
    sandboxConfigurations,
    configuration,
    chat: {
      approvals: chatApprovals,
      steer: chatSteer,
      attachments: chatAttachments,
      suggestions: chatSuggestions,
      artifacts: chatArtifacts,
      stream: async (streamOptions: Parameters<typeof consumeChatStream>[1], onEvent: Parameters<typeof consumeChatStream>[2]) => {
        const streamRequest = buildChatStreamRequest(streamOptions);
        return consumeChatTransport(streamRequest, onEvent);
      },
      continueStream: async (sessionId: string, messageId: string, onEvent: Parameters<typeof consumeChatStream>[2], signal?: AbortSignal) => {
        const query = new URLSearchParams({ message_id: messageId });
        return consumeChatTransport({
          method: 'GET',
          path: `/api/v1/sessions/continue-stream/${encodeURIComponent(sessionId)}?${query.toString()}`,
          headers: { accept: 'text/event-stream' },
          signal,
        }, onEvent);
      },
      stop: async (sessionId: string, messageId: string, signal?: AbortSignal) => {
        return parseActionSuccessResponse(await request({
          method: 'POST',
          path: `/api/v1/sessions/${encodeURIComponent(sessionId)}/stop`,
          body: { message_id: messageId },
          signal,
        }));
      },
    },
  };
}

export type WeKnoraClient = ReturnType<typeof createWeKnoraClient>;
