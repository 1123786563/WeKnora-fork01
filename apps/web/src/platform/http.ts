import { createJsonTransport, type FetchLike } from '@weknora/api-client';
import type { Credential } from '@weknora/api-client';

export interface BrowserTransportOptions {
  fetcher?: FetchLike;
  credential?: Credential;
  tenantId?: string | null;
  locale?: string;
  requestId?: () => string;
}

function authHeader(credential: Credential | undefined): string | undefined {
  if (!credential || credential.kind === 'anonymous') return undefined;
  return credential.kind === 'bearer' ? `Bearer ${credential.accessToken}` : `Embed ${credential.token}`;
}

function defaultRequestId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID();
  return `weknora-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function createBrowserTransport(options: BrowserTransportOptions = {}) {
  const fetcher = options.fetcher ?? ((input, init) => fetch(input, init));
  const base = createJsonTransport(fetcher);
  return {
    async send(request: Parameters<typeof base.send>[0]) {
      const headers = { ...request.headers };
      const authorization = authHeader(options.credential);
      if (authorization) headers.authorization = authorization;
      if (options.credential?.kind !== 'embed' && options.tenantId) headers['x-tenant-id'] = options.tenantId;
      if (options.credential?.kind === 'embed') {
        if (options.credential.sessionSig) headers['x-embed-session'] = options.credential.sessionSig;
        if (options.credential.visitorId) headers['x-embed-visitor'] = options.credential.visitorId;
      }
      if (options.locale) headers['accept-language'] = options.locale;
      headers['x-request-id'] ??= options.requestId?.() ?? defaultRequestId();
      return base.send({ ...request, headers });
    },
  };
}
