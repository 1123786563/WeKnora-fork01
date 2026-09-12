import { createJsonTransport, type FetchLike } from '@weknora/api-client';
import type { Credential } from '@weknora/api-client';
import type { HttpRequest, HttpResult, HttpStreamResult } from '@weknora/api-client';

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
      return sendWithRefresh(request);
    },
    async sendBinary(request: Parameters<typeof base.send>[0]) {
      return sendBinaryWithRefresh(request);
    },
    async sendStream(request: Parameters<typeof base.send>[0]) {
      if (!base.sendStream) throw new Error('Streaming transport is unavailable');
      return sendStreamWithRefresh(request);
    },
  };
}
