import { ApiError, createJsonTransport, type FetchLike } from '@weknora/api-client';
import type { Credential } from '@weknora/api-client';
import type { HttpRequest, HttpResult, HttpStreamResult, NativeMultipartFileRequest } from '@weknora/api-client';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';

// Vue request.ts:137-139 — a request that never got a response (network
// failure) rejects with the localized error.networkError copy instead of the
// raw fetch error, on every page.
function activeLocale(): Locale {
  const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('locale') : null;
  if (stored && isLocale(stored)) return stored;
  return 'zh-CN';
}

function withNetworkError<T>(task: () => Promise<T>): Promise<T> {
  return task().catch((error: unknown) => {
    if (error instanceof ApiError) throw error;
    if (error instanceof TypeError) {
      throw new ApiError({ code: 'NETWORK_ERROR', message: formatMessage(activeLocale(), 'error.networkError'), cause: error });
    }
    throw error;
  });
}

export interface BrowserTransportOptions {
  fetcher?: FetchLike;
  credential?: Credential | (() => Credential);
  tenantId?: string | null | (() => string | null);
  locale?: string;
  requestId?: () => string;
  shouldRefresh?: (request: HttpRequest) => boolean;
  refresh?: () => Promise<void>;
}

function authHeader(credential: Credential | undefined): string | undefined {
  if (!credential || credential.kind === 'anonymous') return undefined;
  return credential.kind === 'bearer' ? `Bearer ${credential.accessToken}` : `Embed ${credential.token}`;
}

function defaultRequestId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID();
  return `weknora-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function isIdempotentRead(method: string): boolean {
  return ['GET', 'HEAD', 'OPTIONS'].includes(method.toUpperCase());
}

type UploadProgressListener = (progress: { loaded: number; total: number }) => void;

/**
 * Observers for in-flight multipart uploads, keyed by the blob: object URL
 * the UI created for its File (the api-client bridge passes
 * NativeFileSource through untouched, so the URI the UI minted is the URI
 * the transport reads). client.ts deliberately stays progress-free; this
 * registry is the web bridge between UI code and the transport onProgress
 * sink.
 */
const uploadProgressListeners = new Map<string, UploadProgressListener>();

/** Observe byte-level progress of the upload sourced from uri. One observer
 * per uri; the transport consumes it when the upload reads the source. */
export function observeUploadProgress(uri: string, listener: UploadProgressListener): () => void {
  uploadProgressListeners.set(uri, listener);
  return () => {
    if (uploadProgressListeners.get(uri) === listener) uploadProgressListeners.delete(uri);
  };
}

/** Currently registered listener for uri (used by the transport and tests). */
export function uploadProgressListener(uri: string): UploadProgressListener | undefined {
  return uploadProgressListeners.get(uri);
}

export function createBrowserTransport(options: BrowserTransportOptions = {}) {
  const fetcher = options.fetcher ?? ((input, init) => fetch(input, init));
  const base = createJsonTransport(fetcher);
  let refreshInFlight: Promise<void> | undefined;

  function currentCredential(): Credential | undefined {
    return typeof options.credential === 'function' ? options.credential() : options.credential;
  }

  function currentTenantId(): string | null | undefined {
    return typeof options.tenantId === 'function' ? options.tenantId() : options.tenantId;
  }

  function decorate(request: HttpRequest, credential = currentCredential()) {
    const headers = { ...request.headers };
    const authorization = authHeader(credential);
    if (authorization) headers.authorization = authorization;
    const tenantId = currentTenantId();
    if (credential?.kind !== 'embed' && tenantId) headers['x-tenant-id'] = tenantId;
    if (credential?.kind === 'embed') {
      if (credential.sessionSig) headers['x-embed-session'] = credential.sessionSig;
      if (credential.visitorId) headers['x-embed-visitor'] = credential.visitorId;
    }
    if (options.locale) headers['accept-language'] = options.locale;
    headers['x-request-id'] ??= options.requestId?.() ?? defaultRequestId();
    return { ...request, headers };
  }

  function canRefresh(request: HttpRequest, credential: Credential | undefined): boolean {
    return credential?.kind === 'bearer'
      && Boolean(credential.refreshToken)
      && Boolean(options.refresh)
      && isIdempotentRead(request.method)
      && (options.shouldRefresh?.(request) ?? true);
  }

  async function refreshOnce(): Promise<void> {
    if (!options.refresh) return;
    if (!refreshInFlight) {
      const task = options.refresh();
      let shared!: Promise<void>;
      shared = task.finally(() => {
        if (refreshInFlight === shared) refreshInFlight = undefined;
      });
      refreshInFlight = shared;
    }
    await refreshInFlight;
  }

  async function sendWithRefresh(request: HttpRequest): Promise<HttpResult> {
    const credential = currentCredential();
    let result = await base.send(decorate(request, credential));
    if (result.status !== 401 || !canRefresh(request, credential)) return result;

    const latest = currentCredential();
    if (credential?.kind === 'bearer' && latest?.kind === 'bearer' && latest.accessToken !== credential.accessToken) {
      return base.send(decorate(request, latest));
    }
    await refreshOnce();
    result = await base.send(decorate(request));
    return result;
  }

  async function sendBinaryWithRefresh(request: HttpRequest): Promise<HttpResult> {
    if (!base.sendBinary) throw new Error('Binary transport is unavailable');
    const credential = currentCredential();
    let result = await base.sendBinary(decorate(request, credential));
    if (result.status !== 401 || !canRefresh(request, credential)) return result;

    const latest = currentCredential();
    if (credential?.kind === 'bearer' && latest?.kind === 'bearer' && latest.accessToken !== credential.accessToken) {
      return base.sendBinary(decorate(request, latest));
    }
    await refreshOnce();
    result = await base.sendBinary(decorate(request));
    return result;
  }

  async function sendMultipartFileWithRefresh(request: NativeMultipartFileRequest): Promise<HttpResult> {
    if (!base.sendMultipartFile) throw new Error('Multipart transport is unavailable');
    // The JSON transport reads the upload bytes from the native source URI (a
    // blob: object URL produced by the api-client Blob bridge, or an http(s)
    // URL), builds the FormData with the backend "file" field and switches to
    // its XHR path when an onProgress sink is attached. Uploads are POSTs, so
    // the 401 refresh path would never replay them; the scoped
    // auth/tenant/request-id headers still come from decorate().
    const listener = uploadProgressListeners.get(request.file.uri);
    if (listener) uploadProgressListeners.delete(request.file.uri);
    const headers = decorate(request).headers;
    const scoped: NativeMultipartFileRequest = {
      method: request.method,
      url: request.url,
      headers,
      file: request.file,
      fields: request.fields,
      signal: request.signal,
    };
    return base.sendMultipartFile(listener ? { ...scoped, onProgress: listener } : scoped);
  }

  async function sendStreamWithRefresh(request: HttpRequest): Promise<HttpStreamResult> {
    const credential = currentCredential();
    let result = await base.sendStream!(decorate(request, credential));
    if (result.status !== 401 || !canRefresh(request, credential)) return result;

    const latest = currentCredential();
    if (credential?.kind === 'bearer' && latest?.kind === 'bearer' && latest.accessToken !== credential.accessToken) {
      return base.sendStream!(decorate(request, latest));
    }
    await refreshOnce();
    result = await base.sendStream!(decorate(request));
    return result;
  }

  return {
    async send(request: Parameters<typeof base.send>[0]) {
      return withNetworkError(() => sendWithRefresh(request));
    },
    async sendBinary(request: Parameters<typeof base.send>[0]) {
      return withNetworkError(() => sendBinaryWithRefresh(request));
    },
    async sendStream(request: Parameters<typeof base.send>[0]) {
      if (!base.sendStream) throw new Error('Streaming transport is unavailable');
      return withNetworkError(() => sendStreamWithRefresh(request));
    },
    async sendMultipartFile(request: NativeMultipartFileRequest) {
      return withNetworkError(() => sendMultipartFileWithRefresh(request));
    },
  };
}
