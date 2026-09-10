import { createJsonTransport, type FetchLike } from '@weknora/api-client';
import type { Credential } from '@weknora/api-client';

export interface BrowserTransportOptions {
  fetcher?: FetchLike;
  credential?: Credential | (() => Credential);
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
      const credential = typeof options.credential === 'function' ? options.credential() : options.credential;
      const authorization = authHeader(credential);
      if (authorization) headers.authorization = authorization;
      if (credential?.kind !== 'embed' && options.tenantId) headers['x-tenant-id'] = options.tenantId;
      if (credential?.kind === 'embed') {
        if (credential.sessionSig) headers['x-embed-session'] = credential.sessionSig;
        if (credential.visitorId) headers['x-embed-visitor'] = credential.visitorId;
      }
      if (options.locale) headers['accept-language'] = options.locale;
      headers['x-request-id'] ??= options.requestId?.() ?? defaultRequestId();
      return base.send({ ...request, headers });
    },
  };
}
