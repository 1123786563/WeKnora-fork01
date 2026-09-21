import { createAuthApi, type AuthMe, type AuthSession, type OIDCConfig, type OIDCURL } from '../auth/endpoints.ts';
import { createOIDCApi } from '../auth/oidc.ts';
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileRuntimeRemoteOptions {
  /**
   * 部署 Origin。构造即强校验：必须是绝对 HTTPS URL、无内嵌 user-info、
   * 无 path/query/fragment——非法 Origin 在任何请求发出前同步抛错。
   */
  origin: string;
  /** 复用既有 ClientRequest 通道（createWeKnoraClient().request），本适配器不新建传输。 */
  request: Request;
}

export interface MobileRuntimeRemote {
  passwordLogin(input: { email: string; password: string }): Promise<AuthSession>;
  me(accessToken: string): Promise<AuthMe>;
  oidcConfig(): Promise<OIDCConfig>;
  oidcUrl(redirectURI: string, frontendRedirectURI?: string, codeChallenge?: string): Promise<OIDCURL>;
  oidcExchange(code: string, state: string, codeVerifier?: string): Promise<AuthSession>;
  oidcNativeExchange(input: { code: string; state: string; redirectUri: string; codeVerifier: string }): Promise<{ token: string; refreshToken: string }>;
  refresh(refreshToken: string): Promise<{ access_token: string; refresh_token: string }>;
  /**
   * GET /api/v1/system/capabilities（internal/handler/deployment_capabilities.go，
   * Viewer+，需 Bearer）。只解包 code/msg/data 成功信封并原样返回 data——
   * 协议窗口（protocol_minimum/protocol_maximum）如何收敛成安全面由 Mobile
   * Runtime 的 clientGate 决定，适配器绝不判定。
   */
  deploymentCapabilities(accessToken: string): Promise<Record<string, unknown>>;
}

function requireDeploymentOrigin(origin: string): void {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try {
    parsed = new URL(origin);
  } catch {
    throw new Error(`deployment origin must be an absolute URL: ${origin}`);
  }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.hostname === '') throw new Error('deployment origin must include a host');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
}

function requireAccessToken(accessToken: string): string {
  if (typeof accessToken !== 'string' || accessToken.trim() === '') throw new Error('access token is required');
  return accessToken;
}

function bearerRequest(request: Request, accessToken: string): Request {
  return (input: ClientRequest): Promise<unknown> =>
    request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
}

function record(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(`${label} must be an object`);
  return value as Record<string, unknown>;
}

function nativeCredential(value: { success: boolean; token?: string; refresh_token?: string; message?: string }): { token: string; refreshToken: string } {
  if (value.success !== true) throw new Error(value.message || 'native OIDC exchange failed');
  if (typeof value.token !== 'string' || value.token.trim() === '') throw new Error('native OIDC access token is required');
  if (typeof value.refresh_token !== 'string' || value.refresh_token.trim() === '') throw new Error('native OIDC refresh token is required');
  return { token: value.token, refreshToken: value.refresh_token };
}

export function createMobileRuntimeRemote(options: MobileRuntimeRemoteOptions): MobileRuntimeRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  const auth = createAuthApi(request);
  const oidc = createOIDCApi(request);
  return {
    passwordLogin(input) {
      return auth.login(input);
    },
    async me(accessToken: string) {
      return createAuthApi(bearerRequest(request, requireAccessToken(accessToken))).me();
    },
    oidcConfig() {
      return auth.oidcConfig();
    },
    oidcUrl(redirectURI: string, frontendRedirectURI?: string, codeChallenge?: string) {
      return auth.oidcUrl(redirectURI, frontendRedirectURI, codeChallenge);
    },
    oidcExchange(code: string, state: string, codeVerifier?: string) {
      return auth.oidcExchange(code, state, codeVerifier);
    },
    async oidcNativeExchange(input) {
      return nativeCredential(await oidc.exchangeNative({
        code: input.code, state: input.state, redirect_uri: input.redirectUri, code_verifier: input.codeVerifier,
      }));
    },
    refresh(refreshToken: string) {
      return auth.refresh(refreshToken);
    },
    async deploymentCapabilities(accessToken: string): Promise<Record<string, unknown>> {
      const response = await bearerRequest(request, requireAccessToken(accessToken))({ method: 'GET', path: '/api/v1/system/capabilities' });
      const root = record(response, '/system/capabilities response');
      if (root.code !== 0) {
        throw new Error(typeof root.msg === 'string' && root.msg !== '' ? root.msg : '/system/capabilities.code must be 0');
      }
      return record(root.data, '/system/capabilities.data');
    },
  };
}
