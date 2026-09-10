import { createJsonTransport, type Credential, type FetchLike, type HttpTransport } from '@weknora/api-client';

export interface MobileTransportOptions {
  credential: () => Credential;
  tenantId?: () => string | null;
  locale?: () => string | undefined;
  fetcher?: FetchLike;
}

function authHeader(credential: Credential): string | undefined {
  if (credential.kind === 'bearer') return `Bearer ${credential.accessToken}`;
  if (credential.kind === 'embed') return `Embed ${credential.token}`;
  return undefined;
}

export function createMobileTransport(options: MobileTransportOptions): HttpTransport {
  const fetcher = options.fetcher ?? ((input, init) => fetch(input, init));
  const base = createJsonTransport(fetcher);
  const decorate = (request: Parameters<typeof base.send>[0]) => {
    const headers = { ...request.headers };
    const credential = options.credential();
    const authorization = authHeader(credential);
    if (authorization) headers.authorization = authorization;
    const tenantId = options.tenantId?.();
    if (tenantId && credential.kind !== 'embed') headers['x-tenant-id'] = tenantId;
    const locale = options.locale?.();
    if (locale) headers['accept-language'] = locale;
    return { ...request, headers };
  };
  return {
    send: (request) => base.send(decorate(request)),
    sendStream: base.sendStream ? (request) => base.sendStream!(decorate(request)) : undefined,
  };
}

export function resolveMobileApiBaseUrl(raw: string | undefined): string {
  const value = raw?.trim() ?? '';
  if (!value) return '';
  try {
    const url = new URL(value);
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return '';
    return url.toString().replace(/\/$/, '');
  } catch { return ''; }
}
