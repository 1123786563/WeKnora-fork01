import type { ClientRequest } from '../client.ts';

export interface EmbedChannelPublicConfig {
  channel_id: string;
  name: string;
  display_title?: string;
  knowledge_base_ids?: string[];
  agent_id: string;
  agent_name?: string;
  agent_avatar?: string;
  welcome_message: string;
  primary_color?: string;
  page_title?: string;
  header_title_mode?: 'channel' | 'session';
  show_suggested_questions?: boolean;
  widget_position?: 'bottom-right' | 'bottom-left' | 'top-right' | 'top-left';
  allow_web_search?: boolean;
  allow_file_upload?: boolean;
  agent_web_search_enabled?: boolean;
  agent_image_upload_enabled?: boolean;
  default_locale?: string;
}

export interface EmbedApi {
  config(channelId: string, token: string): Promise<{ success: boolean; data?: EmbedChannelPublicConfig }>;
  exchange(channelId: string, publishToken: string): Promise<{ success: boolean; data?: { session_token: string; expires_in: number } }>;
  createSession(channelId: string, token: string): Promise<{ success: boolean; data?: { id: string; sig: string } }>;
  messages(channelId: string, token: string, sessionId: string, limit: number, beforeTime?: string, sessionSig?: string): Promise<{ success: boolean; data?: unknown[] }>;
  messageSuggestions(channelId: string, token: string, sessionId: string, messageId: string, sessionSig: string, visitorId?: string): Promise<unknown>;
  stop(channelId: string, token: string, sessionId: string, messageId: string, sessionSig: string): Promise<unknown>;
}

type ConfigResponse = Awaited<ReturnType<EmbedApi['config']>>;
type ExchangeResponse = Awaited<ReturnType<EmbedApi['exchange']>>;
type SessionResponse = Awaited<ReturnType<EmbedApi['createSession']>>;
type MessagesResponse = Awaited<ReturnType<EmbedApi['messages']>>;

function sessionHeaders(token: string, sessionSig: string, visitorId?: string): Record<string, string> {
  const headers: Record<string, string> = {
    Authorization: `Embed ${token}`,
    'X-Embed-Session': sessionSig,
  };
  if (visitorId?.trim()) headers['X-Embed-Visitor'] = visitorId.trim();
  return headers;
}

const providerFilePattern = /^(?:resource|local|minio|cos|tos|s3|oss|ks3|obs):\/\/\S+$/i;
const storageProviderFilePattern = /^storage:\/\/[0-9A-Za-z_-]+\/(?:resource|local|minio|cos|tos|s3|oss|ks3|obs):\/\/\S+$/i;

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

export function createEmbedApi(request: (input: ClientRequest) => Promise<unknown>): EmbedApi {
  return {
    async config(channelId, token) {
      return await request({ method: 'GET', path: `/api/v1/embed/${encodeURIComponent(channelId)}/config`, headers: { Authorization: `Embed ${token}` } }) as ConfigResponse;
    },
    async exchange(channelId, publishToken) {
      return await request({ method: 'POST', path: `/api/v1/embed/${encodeURIComponent(channelId)}/exchange`, headers: { Authorization: `Embed ${publishToken}` } }) as ExchangeResponse;
    },
    async createSession(channelId, token) {
      return await request({ method: 'POST', path: `/api/v1/embed/${encodeURIComponent(channelId)}/sessions`, headers: { Authorization: `Embed ${token}` }, body: {} }) as SessionResponse;
    },
    async messages(channelId, token, sessionId, limit, beforeTime, sessionSig) {
      const query = new URLSearchParams({ limit: String(limit) });
      if (beforeTime) query.set('before_time', beforeTime);
      return await request({ method: 'GET', path: `/api/v1/embed/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(sessionId)}/load?${query}`, headers: sessionSig ? sessionHeaders(token, sessionSig) : { Authorization: `Embed ${token}` } }) as MessagesResponse;
    },
    async messageSuggestions(channelId, token, sessionId, messageId, sessionSig, visitorId) {
      return await request({ method: 'GET', path: `/api/v1/embed/${encodeURIComponent(channelId)}/sessions/${encodeURIComponent(sessionId)}/messages/${encodeURIComponent(messageId)}/suggestions`, headers: sessionHeaders(token, sessionSig, visitorId) });
    },
    async stop(channelId, token, sessionId, messageId, sessionSig) {
      return await request({ method: 'POST', path: `/api/v1/embed/${encodeURIComponent(channelId)}/sessions/${encodeURIComponent(sessionId)}/stop`, headers: sessionHeaders(token, sessionSig), body: { message_id: messageId } });
    },
  };
}
