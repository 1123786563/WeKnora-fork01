import type { WeKnoraClient } from '@weknora/api-client';

export interface ParsedLogin {
  credential: { kind: 'bearer'; accessToken: string; refreshToken?: string };
  tenantId: string | null;
  user?: Record<string, unknown>;
}

function parseLogin(value: unknown): ParsedLogin {
  if (!value || typeof value !== 'object') throw new Error('Authentication request failed');
  const root = value as Record<string, unknown>;
  if (root.success !== true) throw new Error(typeof root.message === 'string' ? root.message : 'Authentication request failed');
  const data = root.data && typeof root.data === 'object' ? root.data as Record<string, unknown> : root;
  const accessToken = data.token ?? data.access_token;
  const refreshToken = data.refresh_token ?? data.refreshToken;
  if (typeof accessToken !== 'string' || accessToken.trim() === '') throw new Error('access token is required');
  if (typeof refreshToken !== 'string' || refreshToken.trim() === '') throw new Error('refresh token is required');
  const tenant = data.tenant && typeof data.tenant === 'object' ? data.tenant as Record<string, unknown> : null;
  const tenantRaw = tenant?.id ?? data.tenant_id;
  return {
    credential: { kind: 'bearer', accessToken, refreshToken },
    tenantId: tenantRaw === undefined || tenantRaw === null ? null : String(tenantRaw),
    user: data.user && typeof data.user === 'object' ? data.user as Record<string, unknown> : undefined,
  };
}

export interface AuthApi {
  login(email: string, password: string): Promise<ParsedLogin>;
  register(input: { username: string; email: string; password: string }): Promise<{ success: boolean; message?: string }>;
  authConfig(): Promise<{ success: boolean; registration_mode: string; complex_password_enabled: boolean }>;
  currentUser(): Promise<{ success: boolean; tenant_required?: boolean; capabilities?: { can_create_tenant?: boolean }; user?: { id: string }; tenant?: { id: string | number; name: string } | null; message?: string }>;
  pendingInvitations(): Promise<number>;
  logout(): Promise<void>;
  lookupInvite(token: string): Promise<{ success: boolean; data?: { tenant_name?: string; role: string }; message?: string }>;
  registerByInvite(input: { token: string; username: string; email: string; password: string }): Promise<ParsedLogin>;
}

export function createAuthApi(client: WeKnoraClient): AuthApi {
  return {
    async login(email, password) { return parseLogin(await client.request({ method: 'POST', path: '/api/v1/auth/login', body: { email, password } })); },
    async register(input) { return await client.request({ method: 'POST', path: '/api/v1/auth/register', body: input }) as { success: boolean; message?: string }; },
    async authConfig() { return await client.request({ method: 'GET', path: '/api/v1/auth/config' }) as { success: boolean; registration_mode: string; complex_password_enabled: boolean }; },
    async currentUser() { return await client.request({ method: 'GET', path: '/api/v1/auth/me' }) as { success: boolean; tenant_required?: boolean; capabilities?: { can_create_tenant?: boolean }; user?: { id: string }; tenant?: { id: string | number; name: string } | null; message?: string }; },
    async pendingInvitations() { const response = await client.request({ method: 'GET', path: '/api/v1/me/invitations/pending-count' }) as { success: boolean; data?: { pending_count: number } }; return response.success ? Math.max(0, response.data?.pending_count ?? 0) : 0; },
    async logout() { await client.request({ method: 'POST', path: '/api/v1/auth/logout' }); },
    async lookupInvite(token) { return await client.request({ method: 'POST', path: '/api/v1/auth/invitations/lookup', body: { token } }) as { success: boolean; data?: { tenant_name?: string; role: string }; message?: string }; },
    async registerByInvite(input) { return parseLogin(await client.request({ method: 'POST', path: '/api/v1/auth/register-by-invite', body: input })); },
  };
}

export function persistLogin(session: ParsedLogin, storage: Pick<Storage, 'setItem' | 'removeItem'> = window.localStorage): void {
  storage.setItem('weknora_token', session.credential.accessToken);
  if (session.credential.kind === 'bearer' && session.credential.refreshToken) storage.setItem('weknora_refresh_token', session.credential.refreshToken);
  else storage.removeItem('weknora_refresh_token');
  if (session.tenantId) storage.setItem('weknora_selected_tenant_id', session.tenantId);
}
