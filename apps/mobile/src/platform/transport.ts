import { createJsonTransport, type Credential, type FetchLike, type HttpTransport } from '@weknora/api-client';

export interface MobileTransportOptions {
  credential: () => Credential;
  refresh?: () => Promise<Credential>;
  tenantId?: () => string | null;
  isTransitioning?: () => boolean;
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
    const transitioning = options.isTransitioning?.() ?? false;
    const credential: Credential = transitioning ? { kind: 'anonymous' } : options.credential();
    const authorization = authHeader(credential);
    if (authorization) headers.authorization = authorization;
    const tenantId = transitioning ? null : options.tenantId?.();
    if (tenantId && credential.kind !== 'embed') headers['x-tenant-id'] = tenantId;
    const locale = options.locale?.();
    if (locale) headers['accept-language'] = locale;
    return { ...request, headers };
  };
  const isIdempotentRead = (method: string) => ['GET', 'HEAD', 'OPTIONS'].includes(method.toUpperCase());
  return {
    send: async (request) => {
      const result = await base.send(decorate(request));
      if (options.isTransitioning?.()) return result;
      const credential = options.credential();
      if (result.status !== 401 || !options.refresh || credential.kind !== 'bearer' || !isIdempotentRead(request.method)) return result;
      await options.refresh();
      return base.send(decorate(request));
    },
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
