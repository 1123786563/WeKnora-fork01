import { parseActionSuccessResponse, parseMessageSuggestionResponse, type ActionSuccessResponse, type ChatStreamEvent, type MessageSuggestionSet } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';
import { array, encoded, query, record, success, withSignal, type JsonRecord } from '../identity/common.ts';

export type EmbedRequest = (input: ClientRequest) => Promise<unknown>;
export type EmbedStream = (input: ClientRequest, onEvent: (event: ChatStreamEvent) => void, signal?: AbortSignal) => Promise<void>;
export type EmbedPayload = Record<string, unknown>;

export interface EmbedChannel extends EmbedPayload { id: string; name?: string; agent_id?: string; enabled?: boolean }
export interface EmbedPublicConfig extends EmbedPayload { channel_id: string; agent_id: string; welcome_message?: string }
export interface EmbedSession { id: string; signature: string }
export interface EmbedSessionToken { sessionToken: string; expiresIn: number }
export interface IMChannel extends EmbedPayload { id: string; platform: string; enabled?: boolean; agent_id?: string }

function requireString(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}

const secretNames = new Set(['api_key', 'app_secret', 'access_token', 'refresh_token', 'token', 'password', 'secret', 'secret_key', 'bot_secret', 'agent_secret', 'encoding_aes_key']);
function redact(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(redact);
  if (value === null || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value as JsonRecord)
    .filter(([key]) => !secretNames.has(key.toLowerCase()) && !key.toLowerCase().endsWith('_api_key'))
    .map(([key, item]) => [key, redact(item)]));
}

function successData(value: unknown, path: string): unknown {
  return redact(success(value, path).data);
}

function successRecord(value: unknown, path: string): EmbedPayload {
  return record(successData(value, path), `${path}.data`);
}

// List endpoints may legitimately return a null data payload for an empty
// collection; surface that as an empty list instead of a parse error.
function successArray(value: unknown, path: string): EmbedPayload[] {
  return array(successData(value, path), `${path}.data`) as EmbedPayload[];
}

function successList(value: unknown, path: string): EmbedPayload[] {
  const data = successData(value, path);
  return Array.isArray(data) ? (data as EmbedPayload[]) : [];
}

function rawData(value: unknown, path: string): unknown {
  return redact(record(value, path).data);
}

function rawRecord(value: unknown, path: string): EmbedPayload {
  return record(rawData(value, path), `${path}.data`);
}

function rawArray(value: unknown, path: string): EmbedPayload[] {
  return array(rawData(value, path), `${path}.data`) as EmbedPayload[];
}

function action(value: unknown): ActionSuccessResponse {
  return parseActionSuccessResponse(value);
}

export function embedHeaders(token: string, signature?: string, visitorId?: string): Record<string, string> {
  const value = requireString(token, 'token');
  const headers: Record<string, string> = { Authorization: `Embed ${value}` };
  if (signature?.trim()) headers['X-Embed-Session'] = signature;
  if (visitorId?.trim()) headers['X-Embed-Visitor'] = visitorId;
  return headers;
}

export function extractEmbedToken(location: string): string {
  try {
    const url = new URL(location);
    const queryToken = url.searchParams.get('token')?.trim();
    if (queryToken) return queryToken;
    const hash = url.hash.startsWith('#') ? url.hash.slice(1) : url.hash;
    return new URLSearchParams(hash).get('token')?.trim() || '';
  } catch {
    return '';
  }
}

export function createEmbedApi(request: EmbedRequest, stream?: EmbedStream) {
  const publicPath = (channelId: string, suffix: string) => `/api/v1/embed/${encoded(channelId, 'channelId')}${suffix}`;
  const channelsPath = '/api/v1/embed-channels';
  const imPath = '/api/v1/im-channels';

  const publicApi = {
    async exchange(channelId: string, publishToken: string, signal?: AbortSignal): Promise<EmbedSessionToken> {
      const data = successRecord(await request(withSignal({ method: 'POST', path: publicPath(channelId, '/exchange'), headers: embedHeaders(publishToken), body: {} }, signal)), '/embed/exchange');
      return { sessionToken: requireString(data.session_token, '/embed/exchange.data.session_token'), expiresIn: typeof data.expires_in === 'number' ? data.expires_in : 0 };
    },
    async config(channelId: string, token: string, signal?: AbortSignal): Promise<EmbedPublicConfig> {
      return successRecord(await request(withSignal({ method: 'GET', path: publicPath(channelId, '/config'), headers: embedHeaders(token) }, signal)), '/embed/config') as EmbedPublicConfig;
    },
    async createSession(channelId: string, token: string, signal?: AbortSignal): Promise<EmbedSession> {
      const data = successRecord(await request(withSignal({ method: 'POST', path: publicPath(channelId, '/sessions'), headers: embedHeaders(token), body: {} }, signal)), '/embed/sessions');
      return { id: requireString(data.id, '/embed/sessions.data.id'), signature: requireString(data.sig, '/embed/sessions.data.sig') };
    },
    async messages(channelId: string, token: string, sessionId: string, options: { limit: number; beforeTime?: string; signature?: string; visitorId?: string }, signal?: AbortSignal): Promise<EmbedPayload[]> {
      const path = query(publicPath(channelId, `/messages/${encoded(sessionId, 'sessionId')}/load`), [['limit', options.limit], ['before_time', options.beforeTime]]);
      return successArray(await request(withSignal({ method: 'GET', path, headers: embedHeaders(token, options.signature, options.visitorId) }, signal)), '/embed/messages');
    },
    async chat(input: { channelId: string; token: string; sessionId: string; signature?: string; visitorId?: string; mode?: 'knowledge' | 'agent'; body: EmbedPayload; lastEventId?: string; }, onEvent: (event: ChatStreamEvent) => void, signal?: AbortSignal): Promise<void> {
      const mode = input.mode ?? 'knowledge';
      const path = publicPath(input.channelId, `/${mode === 'agent' ? 'agent-chat' : 'knowledge-chat'}/${encoded(input.sessionId, 'sessionId')}`);
      const streamRequest: ClientRequest = {
        method: 'POST',
        path,
        headers: {
          accept: 'text/event-stream',
          ...embedHeaders(input.token, input.signature, input.visitorId),
          ...(input.lastEventId ? { 'Last-Event-ID': input.lastEventId } : {}),
        },
        body: input.body,
        ...(signal === undefined ? {} : { signal }),
      };
      if (!stream) throw new Error('embed stream transport is not configured');
      await stream(streamRequest, onEvent, signal);
    },
    async stop(channelId: string, token: string, sessionId: string, messageId: string, signature: string, visitorId?: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
      return action(await request(withSignal({ method: 'POST', path: publicPath(channelId, `/sessions/${encoded(sessionId, 'sessionId')}/stop`), headers: embedHeaders(token, signature, visitorId), body: { message_id: messageId } }, signal)));
    },
    async suggestedQuestions(channelId: string, token: string, limit?: number, signal?: AbortSignal): Promise<EmbedPayload[]> {
      const path = query(publicPath(channelId, '/suggested-questions'), [['limit', limit && limit > 0 ? limit : undefined]]);
      const data = successRecord(await request(withSignal({ method: 'GET', path, headers: embedHeaders(token) }, signal)), '/embed/suggested-questions');
      return array(data.questions, '/embed/suggested-questions.data.questions') as EmbedPayload[];
    },
    async ensureMessageSuggestions(channelId: string, token: string, sessionId: string, messageId: string, signature: string, visitorId: string, regenerate = false, signal?: AbortSignal): Promise<MessageSuggestionSet> {
      return parseMessageSuggestionResponse(await request(withSignal({ method: 'POST', path: publicPath(channelId, `/sessions/${encoded(sessionId, 'sessionId')}/messages/${encoded(messageId, 'messageId')}/suggestions`), headers: embedHeaders(token, signature, visitorId), body: { regenerate } }, signal)));
    },
    async messageSuggestions(channelId: string, token: string, sessionId: string, messageId: string, signature: string, visitorId: string, signal?: AbortSignal): Promise<MessageSuggestionSet> {
      return parseMessageSuggestionResponse(await request(withSignal({ method: 'GET', path: publicPath(channelId, `/sessions/${encoded(sessionId, 'sessionId')}/messages/${encoded(messageId, 'messageId')}/suggestions`), headers: embedHeaders(token, signature, visitorId) }, signal)));
    },
    async recordMessageSuggestionEvent(channelId: string, token: string, sessionId: string, signature: string, visitorId: string, suggestionSetId: string, eventType: 'impression' | 'click' | 'dismiss', questionId = '', signal?: AbortSignal): Promise<void> {
      const response = await request(withSignal({ method: 'POST', path: publicPath(channelId, `/sessions/${encoded(sessionId, 'sessionId')}/suggestion-events`), headers: embedHeaders(token, signature, visitorId), body: { suggestion_set_id: suggestionSetId, question_id: questionId, event_type: eventType } }, signal));
      if (response !== undefined) parseActionSuccessResponse(response);
    },
    async chunk(channelId: string, token: string, chunkId: string, signal?: AbortSignal): Promise<EmbedPayload> {
      return successRecord(await request(withSignal({ method: 'GET', path: publicPath(channelId, `/chunks/${encoded(chunkId, 'chunkId')}`), headers: embedHeaders(token) }, signal)), '/embed/chunks');
    },
  };

  const channels = {
    async listByAgent(agentId: string, signal?: AbortSignal): Promise<EmbedChannel[]> {
      return successArray(await request(withSignal({ method: 'GET', path: `/api/v1/agents/${encoded(agentId, 'agentId')}/embed-channels` }, signal)), '/embed-channels') as EmbedChannel[];
    },
    async listAll(signal?: AbortSignal): Promise<EmbedChannel[]> {
      return successList(await request(withSignal({ method: 'GET', path: channelsPath }, signal)), channelsPath) as EmbedChannel[];
    },
    async create(agentId: string, input: EmbedPayload, signal?: AbortSignal): Promise<EmbedChannel> {
      return successRecord(await request(withSignal({ method: 'POST', path: `/api/v1/agents/${encoded(agentId, 'agentId')}/embed-channels`, body: input }, signal)), '/embed-channels') as EmbedChannel;
    },
    async get(id: string, signal?: AbortSignal): Promise<EmbedChannel> {
      return successRecord(await request(withSignal({ method: 'GET', path: `${channelsPath}/${encoded(id, 'channelId')}` }, signal)), channelsPath) as EmbedChannel;
    },
    async update(id: string, input: EmbedPayload, signal?: AbortSignal): Promise<EmbedChannel> {
      return successRecord(await request(withSignal({ method: 'PUT', path: `${channelsPath}/${encoded(id, 'channelId')}`, body: input }, signal)), channelsPath) as EmbedChannel;
    },
    async remove(id: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
      return action(await request(withSignal({ method: 'DELETE', path: `${channelsPath}/${encoded(id, 'channelId')}` }, signal)));
    },
    async rotateToken(id: string, signal?: AbortSignal): Promise<EmbedChannel> {
      return successRecord(await request(withSignal({ method: 'POST', path: `${channelsPath}/${encoded(id, 'channelId')}/rotate-token`, body: {} }, signal)), channelsPath) as EmbedChannel;
    },
    async previewSession(id: string, signal?: AbortSignal): Promise<EmbedSessionToken> {
      const data = successRecord(await request(withSignal({ method: 'POST', path: `${channelsPath}/${encoded(id, 'channelId')}/preview-session`, body: {} }, signal)), channelsPath);
      return { sessionToken: requireString(data.session_token, '/embed-channels/preview-session.data.session_token'), expiresIn: typeof data.expires_in === 'number' ? data.expires_in : 0 };
    },
    async stats(id: string, signal?: AbortSignal): Promise<EmbedPayload> {
      return successRecord(await request(withSignal({ method: 'GET', path: `${channelsPath}/${encoded(id, 'channelId')}/stats` }, signal)), channelsPath);
    },
  };

  const im = {
    async listByAgent(agentId: string, signal?: AbortSignal): Promise<IMChannel[]> {
      return rawArray(await request(withSignal({ method: 'GET', path: `/api/v1/agents/${encoded(agentId, 'agentId')}/im-channels` }, signal)), '/im-channels') as IMChannel[];
    },
    async listAll(signal?: AbortSignal): Promise<IMChannel[]> {
      const envelope = record(await request(withSignal({ method: 'GET', path: imPath }, signal)), imPath);
      return Array.isArray(envelope.data) ? (envelope.data as IMChannel[]) : [];
    },
    async create(agentId: string, input: EmbedPayload, signal?: AbortSignal): Promise<IMChannel> {
      return rawRecord(await request(withSignal({ method: 'POST', path: `/api/v1/agents/${encoded(agentId, 'agentId')}/im-channels`, body: input }, signal)), '/im-channels') as IMChannel;
    },
    async update(id: string, input: EmbedPayload, signal?: AbortSignal): Promise<IMChannel> {
      return rawRecord(await request(withSignal({ method: 'PUT', path: `${imPath}/${encoded(id, 'channelId')}`, body: input }, signal)), '/im-channels') as IMChannel;
    },
    async remove(id: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
      return action(await request(withSignal({ method: 'DELETE', path: `${imPath}/${encoded(id, 'channelId')}` }, signal)));
    },
    async toggle(id: string, signal?: AbortSignal): Promise<IMChannel> {
      return rawRecord(await request(withSignal({ method: 'POST', path: `${imPath}/${encoded(id, 'channelId')}/toggle` }, signal)), '/im-channels') as IMChannel;
    },
    wechat: {
      async qrCode(signal?: AbortSignal): Promise<EmbedPayload> {
        return rawRecord(await request(withSignal({ method: 'POST', path: '/api/v1/wechat/qrcode', body: {} }, signal)), '/wechat/qrcode');
      },
      async status(qrcode: string, signal?: AbortSignal): Promise<EmbedPayload> {
        return rawRecord(await request(withSignal({ method: 'POST', path: '/api/v1/wechat/qrcode/status', body: { qrcode } }, signal)), '/wechat/qrcode/status');
      },
    },
  };

  return { public: publicApi, channels, im };
}

const providerFilePattern = /^(?:resource|local|minio|cos|tos|s3|oss|ks3|obs):\/\/\S+$/i;
const storageProviderFilePattern = /^storage:\/\/[0-9A-Za-z_-]+\/(?:resource|local|minio|cos|tos|s3|oss|ks3|obs):\/\/\S+$/i;

/** Build a channel-scoped protected file request using embed credentials
 * (HEAD lineage API kept for embed file downloads). */
export function buildEmbedFileRequest(channelId: string, token: string, filePath: string): ClientRequest | null {
  const cleanChannelId = channelId.trim();
  const cleanToken = token.trim();
  const cleanFilePath = filePath.trim();
  if (!cleanChannelId || !cleanToken || (!providerFilePattern.test(cleanFilePath) && !storageProviderFilePattern.test(cleanFilePath))) return null;
  const query = new URLSearchParams({ file_path: cleanFilePath }).toString();
  return {
    method: 'GET',
    path: `/api/v1/embed/${encodeURIComponent(cleanChannelId)}/files?${query}`,
    headers: { Authorization: `Embed ${cleanToken}` },
  };
}

export type EmbedApi = ReturnType<typeof createEmbedApi>;
