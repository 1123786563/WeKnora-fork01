import type { ClientRequest } from '../client.ts';

export interface OIDCAuthURLResponse {
  success: boolean;
  authorization_url?: string;
  state?: string;
  provider_display_name?: string;
  message?: string;
}

export interface OIDCConfigResponse {
  success: boolean;
  enabled: boolean;
  provider_display_name?: string;
  message?: string;
}

/** Exact JSON shape emitted by internal/handler/dto.AuthOIDCCallbackResponse. */
export interface OIDCExchangeResponse {
  success: boolean;
  message?: string;
  user?: Record<string, unknown>;
  tenant?: Record<string, unknown> | null;
  memberships?: Array<Record<string, unknown>>;
  token?: string;
  refresh_token?: string;
  is_new_user?: boolean;
}

export type AuthRequest = (input: ClientRequest) => Promise<unknown>;

export interface NativeOIDCExchangeRequest { code: string; state: string; redirect_uri: string; code_verifier: string }

function requireNonEmpty(value: string, field: string): string {
  const trimmed = value.trim();
  if (!trimmed) throw new Error(`${field} is required`);
  return trimmed;
}

export function createOIDCApi(request: AuthRequest) {
  return {
    async config(): Promise<OIDCConfigResponse> {
      return await request({ method: 'GET', path: '/api/v1/auth/oidc/config' }) as OIDCConfigResponse;
    },
    async start(redirectURI: string): Promise<OIDCAuthURLResponse> {
      const query = new URLSearchParams({ redirect_uri: redirectURI }).toString();
      return await request({ method: 'GET', path: `/api/v1/auth/oidc/url?${query}` }) as OIDCAuthURLResponse;
    },
    async startNative(redirectURI: string, codeChallenge: string, frontendRedirectURI = 'weknora://oidc'): Promise<OIDCAuthURLResponse> {
      const query = new URLSearchParams({
        redirect_uri: redirectURI,
        frontend_redirect_uri: requireNonEmpty(frontendRedirectURI, 'frontend_redirect_uri'),
        // This challenge binds the server-issued one-time application code.
        // The provider callback remains server-owned and therefore does not
        // receive the native scheme as its redirect_uri.
        code_challenge: requireNonEmpty(codeChallenge, 'code_challenge'),
      }).toString();
      return await request({ method: 'GET', path: `/api/v1/auth/oidc/url?${query}` }) as OIDCAuthURLResponse;
    },
    /** Browser OIDC remains deliberately unavailable to native callers. */
    async exchange(_code: string, _state: string, _redirectURI: string): Promise<OIDCExchangeResponse> {
      throw new Error('OIDC_EXCHANGE_UNAVAILABLE');
    },
    async exchangeNative(input: NativeOIDCExchangeRequest): Promise<OIDCExchangeResponse> {
      const body = {
        code: requireNonEmpty(input.code, 'code'),
        state: requireNonEmpty(input.state, 'state'),
        redirect_uri: requireNonEmpty(input.redirect_uri, 'redirect_uri'),
        code_verifier: requireNonEmpty(input.code_verifier, 'code_verifier'),
      };
      const response = await request({ method: 'POST', path: '/api/v1/auth/mobile/exchange', body });
      if (!response || typeof response !== 'object') throw new Error('OIDC_EXCHANGE_INVALID_RESPONSE');
      return response as OIDCExchangeResponse;
    },
  };
}
