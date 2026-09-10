import { parseKnowledgeBaseListResponse, type KnowledgeBase } from '@weknora/contracts';
import { ApiError, errorFromResult } from './errors.ts';
import type { HttpRequest, HttpResult, HttpTransport } from './ports.ts';

export interface ClientRequest {
  method: string;
  path: string;
  headers?: Record<string, string>;
  body?: unknown;
  signal?: AbortSignal;
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

function joinURL(baseURL: string, path: string): string {
  const base = baseURL.replace(/\/+$/, '');
  const suffix = path.startsWith('/') ? path : `/${path}`;
  return `${base}${suffix}`;
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
      body: input.body,
      signal: controller.signal,
    };
    try {
      const result: HttpResult = await options.transport.send(request);
      if (controller.signal.aborted) {
        throw new DOMException('Request was cancelled', 'AbortError');
      }
      if (result.status === 204) return undefined;
      if (result.status < 200 || result.status >= 300) throw errorFromResult(result.status, result.body, result.headers);
      return result.body;
    } catch (error: unknown) {
      if (error instanceof ApiError) throw error;
      if (timedOut || (error instanceof DOMException && error.name === 'TimeoutError')) {
        throw new ApiError({ code: 'TIMEOUT', message: 'Request timed out', cause: error });
      }
      if (input.signal?.aborted || (error instanceof DOMException && error.name === 'AbortError')) {
        throw new ApiError({ code: 'CANCELLED', message: 'Request was cancelled', cause: error });
      }
      throw error;
    } finally {
      clearTimeout(timer);
      input.signal?.removeEventListener('abort', cancel);
    }
  }

  return {
    request,
    knowledgeBases: {
      async list(params: KnowledgeBaseListParams = {}): Promise<KnowledgeBase[]> {
        const path = withQuery('/api/v1/knowledge-bases', { ...params });
        return parseKnowledgeBaseListResponse(await request({ method: 'GET', path }));
      },
    },
  };
}

export type WeKnoraClient = ReturnType<typeof createWeKnoraClient>;
