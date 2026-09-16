import type { ClientRequest } from '../client.ts';

export interface LoginInput { email: string; password: string }
export interface RegisterInput { username: string; email: string; password: string }
export interface RegistrationResult { user: Record<string, unknown>; tenant?: Record<string, unknown> | null }
export interface RegistrationConfig { registrationMode: string; complexPasswordEnabled: boolean }
export interface OIDCConfig { enabled: boolean; providerDisplayName?: string }
export interface OIDCURL { authorizationUrl: string; state: string }
export interface AcceptInvitationByTokenResult {
  membership: { tenantId: number };
  tenantName?: string;
}
export interface InvitationLookup { tenantId: number; tenantName?: string; role: string; expiresAt: string }
export interface AuthSession {
  token: string;
  refreshToken: string;
  user?: Record<string, unknown>;
  tenant?: Record<string, unknown> | null;
  memberships?: unknown[];
}
export interface ParsedLogin {
  credential: { kind: 'bearer'; accessToken: string; refreshToken?: string };
  tenantId?: string;
  user?: Record<string, unknown>;
  tenant?: Record<string, unknown> | null;
}

export function parseLogin(value: unknown): ParsedLogin {
  const root = successEnvelope(value);
  const data = root.data && typeof root.data === 'object' ? record(root.data, 'auth data') : root;
  const token = requiredString(data.token ?? data.access_token, 'access token');
  const refreshToken = data.refresh_token ?? data.refreshToken;
  if (refreshToken !== undefined && typeof refreshToken !== 'string') throw new Error('refresh token must be a string');
  const tenant = data.tenant && typeof data.tenant === 'object' ? record(data.tenant, 'tenant') : data.tenant === null ? null : undefined;
  return { credential: { kind: 'bearer', accessToken: token, ...(refreshToken === undefined ? {} : { refreshToken }) }, tenantId: tenant?.id === undefined ? undefined : String(tenant.id), user: data.user && typeof data.user === 'object' ? record(data.user, 'user') : undefined, tenant };
}
export interface AuthUser extends Record<string, unknown> { id: string }
export interface AuthTenant extends Record<string, unknown> { id: string | number }
export interface AuthMe {
  user: AuthUser;
  tenant?: AuthTenant | null;
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

function recordWithId(value: unknown, label: string): Record<string, unknown> & { id: string } {
  const result = record(value, label);
  return { ...result, id: requiredString(result.id, `${label}.id`) };
}

function tenantRecord(value: unknown, label: string): AuthTenant {
  const result = record(value, label);
  const id = result.id;
  if ((typeof id !== 'string' || id.trim() === '') && (typeof id !== 'number' || !Number.isSafeInteger(id) || id <= 0)) {
    throw new Error(`${label}.id is required`);
  }
  return { ...result, id: typeof id === 'string' ? id : id as number };
}

function requiredBoolean(value: unknown, label: string): boolean {
  if (typeof value !== 'boolean') throw new Error(`${label} must be a boolean`);
  return value;
}

function requiredSafeInteger(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) throw new Error(`${label} must be a safe integer`);
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
    tenant: data.tenant === null ? null : data.tenant && typeof data.tenant === 'object' ? record(data.tenant, 'tenant') : data.active_tenant && typeof data.active_tenant === 'object' ? record(data.active_tenant, 'active tenant') : undefined,
    memberships: Array.isArray(data.memberships) ? data.memberships : undefined,
  };
}

export function createAuthApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async login(input: LoginInput): Promise<AuthSession> {
      return parseSession(await request({ method: 'POST', path: '/api/v1/auth/login', body: input }));
    },
    async register(input: RegisterInput): Promise<RegistrationResult> {
      const root = successEnvelope(await request({ method: 'POST', path: '/api/v1/auth/register', body: input }));
      return {
        user: record(root.user, 'registration user'),
        tenant: root.tenant === null ? null : root.tenant === undefined ? undefined : record(root.tenant, 'registration tenant'),
      };
    },
    async registrationConfig(): Promise<RegistrationConfig> {
      const root = successEnvelope(await request({ method: 'GET', path: '/api/v1/auth/config' }));
      return {
        registrationMode: requiredString(root.registration_mode, 'registration mode'),
        complexPasswordEnabled: requiredBoolean(root.complex_password_enabled, 'complex password enabled'),
      };
    },
    async oidcConfig(): Promise<OIDCConfig> {
      const root = successEnvelope(await request({ method: 'GET', path: '/api/v1/auth/oidc/config' }));
      return {
        enabled: requiredBoolean(root.enabled, 'OIDC enabled'),
        providerDisplayName: root.provider_display_name === undefined ? undefined : requiredString(root.provider_display_name, 'OIDC provider display name'),
      };
    },
    async oidcUrl(redirectURI: string, frontendRedirectURI?: string, codeChallenge?: string): Promise<OIDCURL> {
      const queryParams = new URLSearchParams({ redirect_uri: redirectURI });
      if (frontendRedirectURI !== undefined) queryParams.set('frontend_redirect_uri', frontendRedirectURI);
      if (codeChallenge !== undefined) queryParams.set('code_challenge', codeChallenge);
      const query = queryParams.toString();
      const root = successEnvelope(await request({ method: 'GET', path: `/api/v1/auth/oidc/url?${query}` }));
      return {
        authorizationUrl: requiredString(root.authorization_url, 'authorization URL'),
        state: requiredString(root.state, 'OIDC state'),
      };
    },
    async oidcExchange(code: string, state: string, codeVerifier?: string): Promise<AuthSession> {
      return parseSession(await request({ method: 'POST', path: '/api/v1/auth/oidc/exchange', body: { code, state, ...(codeVerifier === undefined ? {} : { code_verifier: codeVerifier }) } }));
    },
    async autoSetup(): Promise<AuthSession> {
      return parseSession(await request({ method: 'POST', path: '/api/v1/auth/auto-setup', body: {} }));
    },
    async switchTenant(tenantId: number, refreshToken?: string): Promise<AuthSession> {
      if (!Number.isSafeInteger(tenantId) || tenantId <= 0) throw new Error('tenantId must be a positive safe integer');
      return parseSession(await request({ method: 'POST', path: '/api/v1/auth/switch-tenant', body: { tenant_id: tenantId, ...(refreshToken === undefined ? {} : { refresh_token: refreshToken }) } }));
    },
    async lookupInvitation(token: string): Promise<InvitationLookup> {
      const root = successEnvelope(await request({ method: 'POST', path: '/api/v1/auth/invitations/lookup', body: { token } }));
      const data = record(root.data, 'invitation lookup data');
      return {
        tenantId: requiredSafeInteger(data.tenant_id, 'tenantId'),
        tenantName: data.tenant_name === undefined ? undefined : requiredString(data.tenant_name, 'tenant name'),
        role: requiredString(data.role, 'invitation role'),
        expiresAt: requiredString(data.expires_at, 'invitation expiry'),
      };
    },
    /** Port of Vue acceptInvitationByToken (POST /api/v1/me/invitations/accept-by-token,
     *  internal/router/routes_auth_tenant.go:175). Authenticated; returns the new
     *  membership so the caller can refresh and switch scope. */
    async acceptInvitationByToken(token: string): Promise<AcceptInvitationByTokenResult> {
      if (typeof token !== 'string' || token.trim() === '') throw new Error('token is required');
      const root = successEnvelope(await request({ method: 'POST', path: '/api/v1/me/invitations/accept-by-token', body: { token } }));
      const data = root.data && typeof root.data === 'object' && !Array.isArray(root.data) ? record(root.data, 'accept-by-token data') : root;
      const membership = record(data.membership, 'accept-by-token membership');
      const tenantId = membership.tenant_id;
      if (typeof tenantId !== 'number' || !Number.isSafeInteger(tenantId) || tenantId <= 0) throw new Error('membership.tenant_id must be a positive safe integer');
      return {
        membership: { tenantId },
        tenantName: typeof data.tenant_name === 'string' ? data.tenant_name : undefined,
      };
    },
    async registerByInvite(input: { token: string; email: string; username: string; password: string }): Promise<AuthSession> {
      return parseSession(await request({ method: 'POST', path: '/api/v1/auth/register-by-invite', body: input }));
    },
    async validate(): Promise<{ valid: boolean }> {
      const root = successEnvelope(await request({ method: 'GET', path: '/api/v1/auth/validate' }));
      return { valid: root.valid === undefined ? true : requiredBoolean(root.valid, 'token valid') };
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
      return { user: recordWithId(data.user, 'auth me user'), tenant: data.tenant === null ? null : data.tenant ? tenantRecord(data.tenant, 'auth me tenant') : undefined, memberships: Array.isArray(data.memberships) ? data.memberships : undefined, tenant_required: data.tenant_required === true, capabilities: data.capabilities && typeof data.capabilities === 'object' ? record(data.capabilities, 'capabilities') : undefined };
    },
    async logout(): Promise<void> {
      const response = await request({ method: 'POST', path: '/api/v1/auth/logout', body: {} });
      if (response !== undefined) successEnvelope(response);
    },
  };
}

export type AuthApi = ReturnType<typeof createAuthApi>;
