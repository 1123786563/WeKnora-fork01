import {
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
  signal?: AbortSignal;
}

export interface ChatMessageListParams {
  beforeTime?: string;
  limit?: number;
  signal?: AbortSignal;
}

export function createChatSessionsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async list(params: ChatSessionListParams = {}): Promise<ChatSessionListResponse> {
      const query = new URLSearchParams();
      if (params.page !== undefined) query.set('page', String(params.page));
      if (params.pageSize !== undefined) query.set('page_size', String(params.pageSize));
      if (params.source) query.set('source', params.source);
      const suffix = query.toString();
      return parseChatSessionListResponse(await request({
        method: 'GET',
        path: `/api/v1/sessions${suffix ? `?${suffix}` : ''}`,
        signal: params.signal,
      }));
    },
    async create(input: { title?: string; description?: string } = {}): Promise<ChatSession> {
      return parseChatSessionResponse(await request({ method: 'POST', path: '/api/v1/sessions', body: input }));
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
