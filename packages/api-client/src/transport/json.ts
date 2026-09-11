import type { HttpRequest, HttpResult, HttpTransport } from '../ports.ts';

export interface FetchResponseLike {
  status: number;
  headers: { get(name: string): string | null };
  json(): Promise<unknown>;
  text(): Promise<string>;
  blob?(): Promise<Blob>;
  arrayBuffer?(): Promise<ArrayBuffer>;
  body?: { getReader(): { read(): Promise<{ done: boolean; value?: Uint8Array }>; releaseLock?(): void } } | null;
}

export type FetchLike = (input: string, init?: {
  method?: string;
  headers?: Record<string, string>;
  body?: string | FormData;
  signal?: AbortSignal;
}) => Promise<FetchResponseLike>;

export function createJsonTransport(fetcher: FetchLike): HttpTransport {
  const responseHeaders = (response: FetchResponseLike): Record<string, string> => {
    const headers: Record<string, string> = {};
    for (const name of ['content-type', 'content-disposition', 'content-length', 'x-request-id']) {
      const value = response.headers.get(name);
      if (value) headers[name] = value;
    }
    return headers;
  };

  const readResponse = async (response: FetchResponseLike, binary: boolean): Promise<unknown> => {
    if (response.status === 204) return undefined;
    const contentType = response.headers.get('content-type')?.toLowerCase() ?? '';
    if (!binary || contentType.includes('json')) {
      return contentType.includes('json') ? response.json() : response.text();
    }
    if (contentType.startsWith('text/')) return response.text();
    if (response.blob) return response.blob();
    if (response.arrayBuffer) return response.arrayBuffer();
    return response.text();
  };

  const fetchResponse = async (request: HttpRequest): Promise<FetchResponseLike> => fetcher(request.url, {
    method: request.method,
    headers: request.headers,
    body: typeof FormData !== 'undefined' && request.body instanceof FormData
      ? request.body
      : request.body === undefined ? undefined : JSON.stringify(request.body),
    signal: request.signal,
  });

  return {
    async send(request: HttpRequest): Promise<HttpResult> {
      const response = await fetchResponse(request);
      return { status: response.status, headers: responseHeaders(response), body: await readResponse(response, false) };
    },
    async sendBinary(request: HttpRequest): Promise<HttpResult> {
      const response = await fetchResponse(request);
      return { status: response.status, headers: responseHeaders(response), body: await readResponse(response, true) };
    },
    async sendStream(request: HttpRequest) {
      const response = await fetcher(request.url, {
        method: request.method,
        headers: request.headers,
        body: request.body === undefined ? undefined : JSON.stringify(request.body),
        signal: request.signal,
      });
      const requestId = response.headers.get('x-request-id');
      const headers: Record<string, string> = {};
      if (requestId) headers['x-request-id'] = requestId;
      async function* chunks(): AsyncIterable<string> {
        if (response.body) {
          const reader = response.body.getReader();
          const decoder = new TextDecoder();
          try {
            for (;;) {
              const part = await reader.read();
              if (part.done) break;
              if (part.value) yield decoder.decode(part.value, { stream: true });
            }
            const tail = decoder.decode();
            if (tail) yield tail;
          } finally { reader.releaseLock?.(); }
        } else {
          const text = await response.text();
          if (text) yield text;
        }
      }
      return { status: response.status, headers, chunks: chunks() };
    },
  };
}
