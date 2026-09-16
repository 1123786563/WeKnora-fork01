import type { BearerCredential, HttpTransport } from '../ports.ts';
import type { CredentialAdapter } from '../ports.ts';
import { createRefreshCoordinator } from './refresh-coordinator.ts';

export interface ParsedLogin {
  credential: BearerCredential;
  userId: string;
  tenantId: string | null;
}

export interface ProductIdentitySnapshot {
  userId: string;
  tenantId: string | null;
}

function nonEmpty(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0;
}

export function parseLogin(value: unknown): ParsedLogin {
  if (typeof value !== 'object' || value === null) throw new Error('INVALID_LOGIN');
  const record = value as Record<string, unknown>;
  const user = record.user as Record<string, unknown> | undefined;
  const tenantValueContainer = record.active_tenant ?? record.tenant;
  if (tenantValueContainer !== undefined && tenantValueContainer !== null &&
    (typeof tenantValueContainer !== 'object' || Array.isArray(tenantValueContainer))) throw new Error('INVALID_TENANT_ID');
  const tenant = tenantValueContainer as Record<string, unknown> | null | undefined;
  if (record.success !== true || !nonEmpty(record.token) || !nonEmpty(user?.id)) throw new Error('INVALID_LOGIN');
  if (record.refresh_token !== undefined && !nonEmpty(record.refresh_token)) throw new Error('INVALID_LOGIN');
  const tenantValue = tenant?.id;
  if (tenantValueContainer !== undefined && tenantValueContainer !== null && (tenantValue === undefined || tenantValue === null)) {
    throw new Error('INVALID_TENANT_ID');
  }
  if (tenantValue !== undefined && tenantValue !== null &&
    !((typeof tenantValue === 'string' && /^[1-9]\d*$/.test(tenantValue)) ||
      (typeof tenantValue === 'number' && Number.isSafeInteger(tenantValue) && tenantValue > 0))) {
    throw new Error('INVALID_TENANT_ID');
  }
  return {
    credential: {
      kind: 'bearer',
      accessToken: record.token,
      ...(record.refresh_token === undefined ? {} : { refreshToken: record.refresh_token as string }),
    },
    userId: user!.id as string,
    tenantId: tenantValue == null ? null : String(tenantValue),
  };
}

export interface ProductAuthOptions {
  baseURL: string;
  transport: HttpTransport;
}

export function createProductAuth(options: ProductAuthOptions) {
  const base = options.baseURL.replace(/\/+$/, '');
  async function request(path: string, body: unknown, accessToken?: string): Promise<unknown> {
    const result = await options.transport.send({
      method: 'POST',
      url: `${base}${path}`,
      headers: { accept: 'application/json', 'content-type': 'application/json', ...(accessToken ? { authorization: `Bearer ${accessToken}` } : {}) },
      body,
    });
    if (result.status < 200 || result.status >= 300) throw new Error(`AUTH_HTTP_${result.status}`);
    return result.body;
  }
  async function me(accessToken: string): Promise<ProductIdentitySnapshot> {
    const result = await options.transport.send({ method: 'GET', url: `${base}/api/v1/auth/me`, headers: { accept: 'application/json', authorization: `Bearer ${accessToken}` } });
    if (result.status < 200 || result.status >= 300) throw new Error(`AUTH_HTTP_${result.status}`);
    const root = result.body as Record<string, unknown>;
    if (root.success !== true || !root.data || typeof root.data !== 'object') throw new Error('AUTH_ME_INVALID');
    const data = root.data as Record<string, unknown>;
    const user = data.user as Record<string, unknown> | undefined;
    const tenant = data.tenant as Record<string, unknown> | null | undefined;
    if (typeof user?.id !== 'string' || user.id.trim() === '') throw new Error('AUTH_ME_INVALID');
    if (tenant !== null && tenant !== undefined && (typeof tenant.id !== 'string' && typeof tenant.id !== 'number')) throw new Error('AUTH_ME_INVALID');
    return { userId: user.id, tenantId: tenant == null ? null : String(tenant.id) };
  }
  return {
    request,
    login(email: string, password: string) {
      return request('/api/v1/auth/login', { email, password }).then(parseLogin);
    },
    refresh(refreshToken: string) {
      return request('/api/v1/auth/refresh', { refreshToken });
    },
    me,
  };
}

export function createProductAuthSession(options: ProductAuthOptions & { credentials: CredentialAdapter }) {
  const auth = createProductAuth(options);
  const refreshCoordinator = createRefreshCoordinator({ credentials: options.credentials, refresh: auth.refresh });
  async function request(path: string, body?: unknown): Promise<unknown> {
    const current = await options.credentials.read();
    try {
      return await auth.request(path, body, current.kind === 'bearer' ? current.accessToken : undefined);
    } catch (error) {
      if (!(error instanceof Error) || error.message !== 'AUTH_HTTP_401') throw error;
    }
    if (current.kind !== 'bearer' || !current.refreshToken) throw new Error('AUTH_HTTP_401');
    const refreshed = await refreshCoordinator.refresh();
    return auth.request(path, body, refreshed.accessToken);
  }
  async function me(): Promise<ProductIdentitySnapshot> {
    const current = await options.credentials.read();
    if (current.kind !== 'bearer') throw new Error('AUTH_HTTP_401');
    try { return await auth.me(current.accessToken); } catch (error) {
      if (!(error instanceof Error) || error.message !== 'AUTH_HTTP_401' || !current.refreshToken) throw error;
      const refreshed = await refreshCoordinator.refresh();
      return auth.me(refreshed.accessToken);
    }
  }
  return { ...auth, request, me, refreshCoordinator };
}
