import {
  parseActionSuccessResponse,
  parseChatMessageListResponse,
  parseChatSessionListResponse,
  parseChatSessionResponse,
  type ChatMessage,
  type ChatSession,
  type ChatSessionListResponse,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

export interface ChatSessionListParams {
  page?: number;
  pageSize?: number;
  source?: string;
  keyword?: string;
  agentId?: string;
  signal?: AbortSignal;
}

export interface ChatMessageListParams {
  beforeTime?: string;
  limit?: number;
  signal?: AbortSignal;
}

export interface ChatSessionUpdateInput {
  title: string;
  description?: string;
}

function sessionPath(sessionId: string): string {
  if (typeof sessionId !== 'string' || sessionId.trim() === '') throw new Error('sessionId must not be empty');
  return `/api/v1/sessions/${encodeURIComponent(sessionId)}`;
}

export function createChatSessionsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async list(params: ChatSessionListParams = {}): Promise<ChatSessionListResponse> {
      const query = new URLSearchParams();
      if (params.page !== undefined) query.set('page', String(params.page));
      if (params.pageSize !== undefined) query.set('page_size', String(params.pageSize));
      if (params.source) query.set('source', params.source);
      if (params.keyword) query.set('keyword', params.keyword);
      if (params.agentId) query.set('agent_id', params.agentId);
      const suffix = query.toString();
      return parseChatSessionListResponse(await request({
        method: 'GET',
        path: `/api/v1/sessions${suffix ? `?${suffix}` : ''}`,
        signal: params.signal,
      }));
    },
    /**
     * Fetches one session — the sandbox settings inventory resolves live
     * session ids into titles with it (frontend/src/views/settings/
     * SandboxSettings.vue:332 GET /api/v1/sessions/:id, route
     * internal/router/routes_chat.go:55).
     */
    async get(sessionId: string, signal?: AbortSignal): Promise<ChatSession> {
      return parseChatSessionResponse(await request({ method: 'GET', path: sessionPath(sessionId), ...(signal === undefined ? {} : { signal }) }));
    },
    async create(input: { title?: string; description?: string } = {}): Promise<ChatSession> {
      return parseChatSessionResponse(await request({ method: 'POST', path: '/api/v1/sessions', body: input }));
    },
    async update(sessionId: string, input: ChatSessionUpdateInput, signal?: AbortSignal): Promise<ChatSession> {
      if (typeof input.title !== 'string' || input.title.trim() === '') throw new Error('title must not be empty');
      return parseChatSessionResponse(await request({ method: 'PUT', path: sessionPath(sessionId), body: input, ...(signal === undefined ? {} : { signal }) }));
    },
    async remove(sessionId: string, signal?: AbortSignal): Promise<void> {
      parseActionSuccessResponse(await request({ method: 'DELETE', path: sessionPath(sessionId), ...(signal === undefined ? {} : { signal }) }));
    },
    async batchRemove(sessionIds: readonly string[], signal?: AbortSignal): Promise<void> {
      if (sessionIds.length === 0 || sessionIds.some((id) => typeof id !== 'string' || id.trim() === '')) throw new Error('sessionIds must not be empty');
      parseActionSuccessResponse(await request({ method: 'DELETE', path: '/api/v1/sessions/batch', body: { ids: [...sessionIds] }, ...(signal === undefined ? {} : { signal }) }));
    },
    async clear(sessionId: string, signal?: AbortSignal): Promise<void> {
      parseActionSuccessResponse(await request({ method: 'DELETE', path: `${sessionPath(sessionId)}/messages`, ...(signal === undefined ? {} : { signal }) }));
    },
    async pin(sessionId: string, signal?: AbortSignal): Promise<void> {
      parseActionSuccessResponse(await request({ method: 'POST', path: `${sessionPath(sessionId)}/pin`, ...(signal === undefined ? {} : { signal }) }));
    },
    async unpin(sessionId: string, signal?: AbortSignal): Promise<void> {
      parseActionSuccessResponse(await request({ method: 'DELETE', path: `${sessionPath(sessionId)}/pin`, ...(signal === undefined ? {} : { signal }) }));
    },
    async messages(sessionId: string, params: ChatMessageListParams = {}): Promise<ChatMessage[]> {
      const query = new URLSearchParams();
      if (params.beforeTime) query.set('before_time', params.beforeTime);
      if (params.limit !== undefined) query.set('limit', String(params.limit));
      const suffix = query.toString();
      return parseChatMessageListResponse(await request({
        method: 'GET',
        path: `/api/v1/messages/${encodeURIComponent(sessionId)}/load${suffix ? `?${suffix}` : ''}`,
        signal: params.signal,
      }));
    },
  };
}

export type ChatSessionsApi = ReturnType<typeof createChatSessionsApi>;
