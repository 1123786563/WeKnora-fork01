import type { HttpRequest, HttpResult, HttpTransport, NativeMultipartFileRequest } from '../ports.ts';

/**
 * Structural XHR surface for the multipart upload path. Kept structural (not
 * the DOM lib type) so runtimes without DOM typings still compile.
 */
interface MultipartXhr {
  open(method: string, url: string): void;
  setRequestHeader(name: string, value: string): void;
  send(body?: FormData | null): void;
  abort(): void;
  upload: { onprogress: ((event: { loaded: number; total: number; lengthComputable: boolean }) => void) | null };
  onload: (() => void) | null;
  onerror: (() => void) | null;
  onabort: (() => void) | null;
  status: number;
  responseText: string;
  getAllResponseHeaders(): string;
}

type XhrCtor = new () => MultipartXhr;

function xhrConstructor(): XhrCtor | undefined {
  return (globalThis as { XMLHttpRequest?: XhrCtor }).XMLHttpRequest;
}

function abortError(): Error {
  const error = new Error('The upload was aborted');
  error.name = 'AbortError';
  return error;
}

function revokeUploadSource(uri: string): void {
  if (uri.startsWith('blob:') && typeof URL !== 'undefined' && typeof URL.revokeObjectURL === 'function') {
    URL.revokeObjectURL(uri);
  }
}

/** Sends FormData through XHR so upload.onprogress exposes real byte counts. */
function sendMultipartViaXhr(
  request: NativeMultipartFileRequest,
  form: FormData,
  create: XhrCtor,
): Promise<HttpResult> {
  return new Promise<HttpResult>((resolve, reject) => {
    if (request.signal?.aborted) {
      reject(abortError());
      return;
    }
    const xhr = new create();
    let settled = false;
    const onAbort = (): void => xhr.abort();
    const settle = (fn: () => void): void => {
      if (settled) return;
      settled = true;
      request.signal?.removeEventListener('abort', onAbort);
      fn();
    };
    request.signal?.addEventListener('abort', onAbort, { once: true });
    xhr.open(request.method, request.url);
    for (const [name, value] of Object.entries(request.headers)) {
      // FormData bodies need the browser-generated multipart boundary header.
      if (name.toLowerCase() !== 'content-type') xhr.setRequestHeader(name, value);
    }
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && event.total > 0) request.onProgress?.({ loaded: event.loaded, total: event.total });
    };
    xhr.onload = () => {
      const headers = new Map<string, string>();
      for (const line of xhr.getAllResponseHeaders().split('\r\n')) {
        const separator = line.indexOf(':');
        if (separator > 0) headers.set(line.slice(0, separator).trim().toLowerCase(), line.slice(separator + 1).trim());
      }
      const contentType = (headers.get('content-type') ?? '').toLowerCase();
      let body: unknown;
      if (xhr.status === 204) body = undefined;
      else if (contentType.includes('json')) {
        try { body = JSON.parse(xhr.responseText); } catch { body = xhr.responseText; }
      } else body = xhr.responseText;
      const shim = { get: (name: string) => headers.get(name.toLowerCase()) ?? null };
      const filtered: Record<string, string> = {};
      for (const name of ['content-type', 'content-disposition', 'content-length', 'x-request-id']) {
        const value = shim.get(name);
        if (value) filtered[name] = value;
      }
      settle(() => resolve({ status: xhr.status, headers: filtered, body }));
    };
    xhr.onerror = () => settle(() => reject(new Error('Network request failed')));
    xhr.onabort = () => settle(() => reject(abortError()));
    xhr.send(form);
  });
}

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
    async sendMultipartFile(request: NativeMultipartFileRequest): Promise<HttpResult> {
      const source = await fetcher(request.file.uri, { signal: request.signal });
      if (source.status < 200 || source.status >= 300) throw new Error('unable to read upload source: ' + source.status);
      const blob = source.blob
        ? await source.blob()
        : source.arrayBuffer
          ? new Blob([await source.arrayBuffer()], { type: request.file.type || 'application/octet-stream' })
          : new Blob([await source.text()], { type: request.file.type || 'application/octet-stream' });
      revokeUploadSource(request.file.uri);
      const form = new FormData();
      for (const [name, value] of Object.entries(request.fields ?? {})) form.append(name, value);
      form.append('file', blob, request.file.name || 'file');
      const create = request.onProgress ? xhrConstructor() : undefined;
      if (create) return sendMultipartViaXhr(request, form, create);
      const response = await fetchResponse({
        method: request.method,
        url: request.url,
        headers: request.headers,
        body: form,
        signal: request.signal,
      });
      return { status: response.status, headers: responseHeaders(response), body: await readResponse(response, false) };
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
