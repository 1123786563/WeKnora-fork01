import { parseActionSuccessResponse, parseKnowledgeBaseListResponse, parseKnowledgeBaseResponse, type KnowledgeBase } from '@weknora/contracts';
import { ApiError, createAbortError, errorFromResult, isNamedError } from './errors.ts';
import type { HttpRequest, HttpResult, HttpTransport, NativeFileSource } from './ports.ts';
import { createKnowledgeDocumentsApi } from './knowledge/documents.ts';
import { createKnowledgeFaqApi } from './knowledge/faq.ts';
import { createWikiPagesApi } from './wiki/pages.ts';
import { createDataSourcesApi } from './datasource.ts';
import { createAuthApi } from './auth/endpoints.ts';
import { createChatSessionsApi } from './chat/sessions.ts';
import { createSandboxTerminalApi } from './sandbox/terminal.ts';
import { createConfigurationApi } from './configuration.ts';
import { buildChatStreamRequest, consumeChatStream, consumeStreamResult, createServerSentEventParser, parseChatEvent } from './chat/stream.ts';
import { createChatApprovalsApi } from './chat/approvals.ts';
import { createChatSteerApi } from './chat/steer.ts';
import { createChatAttachmentsApi } from './chat/attachments.ts';
import { createChatSuggestionsApi } from './chat/suggestions.ts';
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
  const auth = createAuthApi(request);
  const sessions = createChatSessionsApi(request);
  const sandbox = createSandboxTerminalApi(request);
  const configuration = createConfigurationApi(request);
  const chatApprovals = createChatApprovalsApi(request);
  const chatSteer = createChatSteerApi(request);
  const chatAttachments = createChatAttachmentsApi(request);
  const chatSuggestions = createChatSuggestionsApi(request);
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
      async remove(id: string): Promise<void> {
        await request({ method: 'DELETE', path: `/api/v1/knowledge-bases/${encodeURIComponent(id)}` });
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
    auth,
    identity,
    administration,
    settings,
    embed,
    sessions,
    sandbox,
    configuration,
    chat: {
      approvals: chatApprovals,
      steer: chatSteer,
      attachments: chatAttachments,
      suggestions: chatSuggestions,
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
