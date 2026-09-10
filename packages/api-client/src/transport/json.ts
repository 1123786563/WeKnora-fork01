import type { HttpRequest, HttpResult, HttpTransport } from '../ports.ts';

export interface FetchResponseLike {
  status: number;
  headers: { get(name: string): string | null };
  json(): Promise<unknown>;
  text(): Promise<string>;
}

export type FetchLike = (input: string, init?: {
  method?: string;
  headers?: Record<string, string>;
  body?: string | FormData;
  signal?: AbortSignal;
}) => Promise<FetchResponseLike>;

export function createJsonTransport(fetcher: FetchLike): HttpTransport {
  return {
    async send(request: HttpRequest): Promise<HttpResult> {
      const response = await fetcher(request.url, {
        method: request.method,
        headers: request.headers,
        body: typeof FormData !== 'undefined' && request.body instanceof FormData
          ? request.body
          : request.body === undefined ? undefined : JSON.stringify(request.body),
        signal: request.signal,
      });
      const contentType = response.headers.get('content-type')?.toLowerCase() ?? '';
      const body = response.status === 204
        ? undefined
        : contentType.includes('json') ? await response.json() : await response.text();
      const requestId = response.headers.get('x-request-id');
      return {
        status: response.status,
        headers: requestId ? { 'x-request-id': requestId } : {},
        body,
      };
    },
  };
}
