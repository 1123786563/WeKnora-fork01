import type { ClientRequest } from '../client.ts';

export interface LoginInput { email: string; password: string }
export interface AuthSession {
  token: string;
  refreshToken: string;
  user?: Record<string, unknown>;
  tenant?: Record<string, unknown> | null;
  memberships?: unknown[];
}
export interface AuthMe {
  user: Record<string, unknown>;
  tenant?: Record<string, unknown> | null;
  memberships?: unknown[];
  tenant_required?: boolean;
  capabilities?: Record<string, unknown>;
}

function record(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(`${label} must be an object`);
  return value as Record<string, unknown>;
}

function requiredString(value: unknown, label: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${label} is required`);
  return value;
}

function successEnvelope(value: unknown): Record<string, unknown> {
  const root = record(value, 'auth response');
  if (root.success !== true) throw new Error(typeof root.message === 'string' ? root.message : 'Authentication request failed');
  return root;
}

function parseSession(value: unknown): AuthSession {
  const root = successEnvelope(value);
  const data = root.data && typeof root.data === 'object' ? record(root.data, 'auth data') : root;
  return {
    token: requiredString(data.token ?? data.access_token, 'access token'),
    refreshToken: requiredString(data.refresh_token ?? data.refreshToken, 'refresh token'),
    user: data.user && typeof data.user === 'object' ? record(data.user, 'user') : undefined,
    tenant: data.tenant === null ? null : data.tenant && typeof data.tenant === 'object' ? record(data.tenant, 'tenant') : undefined,
    memberships: Array.isArray(data.memberships) ? data.memberships : undefined,
  };
}

export function createAuthApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async login(input: LoginInput): Promise<AuthSession> {
      return parseSession(await request({ method: 'POST', path: '/api/v1/auth/login', body: input }));
    },
    async refresh(refreshToken: string): Promise<{ access_token: string; refresh_token: string }> {
      const root = successEnvelope(await request({ method: 'POST', path: '/api/v1/auth/refresh', body: { refreshToken } }));
      return {
        access_token: requiredString(root.access_token ?? (root.data as Record<string, unknown> | undefined)?.access_token, 'access token'),
        refresh_token: requiredString(root.refresh_token ?? (root.data as Record<string, unknown> | undefined)?.refresh_token, 'refresh token'),
      };
    },
    async me(): Promise<AuthMe> {
      const root = successEnvelope(await request({ method: 'GET', path: '/api/v1/auth/me' }));
      const data = record(root.data, 'auth me data');
      return { user: record(data.user, 'auth me user'), tenant: data.tenant === null ? null : data.tenant ? record(data.tenant, 'auth me tenant') : undefined, memberships: Array.isArray(data.memberships) ? data.memberships : undefined, tenant_required: data.tenant_required === true, capabilities: data.capabilities && typeof data.capabilities === 'object' ? record(data.capabilities, 'capabilities') : undefined };
    },
    async logout(): Promise<void> {
      await request({ method: 'POST', path: '/api/v1/auth/logout', body: {} });
    },
  };
}

export type AuthApi = ReturnType<typeof createAuthApi>;
