import type { ScopedVault } from '../vault/scoped-vault.ts';
import type { ResourceRemote } from '../shelf/ports.ts';
import type { Deployment, RuntimeAuthorizedRequest } from './types.ts';

/** Exact credential fields returned by Task 2's `passwordLogin` adapter. */
export interface StoredCredential {
  token: string;
  refreshToken: string;
}

export interface CredentialStore {
  read(deployment: string): Promise<StoredCredential | undefined>;
  write(deployment: string, credential: StoredCredential): Promise<void>;
  clear(deployment: string): Promise<void>;
}

/** Persists only the presentation-safe deployment selection, never credentials. */
export interface DeploymentStore {
  read(): Promise<{ origin: string; label?: string } | undefined>;
  write(deployment: { origin: string; label: string }): Promise<void>;
  clear(): Promise<void>;
}

/** 登记实例清单：只保存 presentation-safe 的 origin/label，永不保存凭据。 */
export interface DeploymentRegistry {
  list(): Promise<Deployment[]>;
  upsert(deployment: Deployment): Promise<void>;
  remove(origin: string): Promise<void>;
}

/** Structural subset of `createMobileRuntimeRemote`; mobile-core remains adapter-independent. */
export interface RuntimeRemote {
  passwordLogin(input: { email: string; password: string }): Promise<StoredCredential>;
  wechatLogin(input: { code: string }): Promise<StoredCredential>;
  me(accessToken: string): Promise<{ user: { id: unknown }; tenant?: { id: unknown } | null; memberships?: unknown[] }>;
  deploymentCapabilities(accessToken: string): Promise<unknown>;
  oidcUrl(redirectUri: string, frontendRedirectUri?: string, codeChallenge?: string): Promise<{ authorizationUrl: string; state: string }>;
  oidcExchange(code: string, state: string, codeVerifier?: string): Promise<StoredCredential>;
  oidcNativeExchange(input: { code: string; state: string; redirectUri: string; codeVerifier: string }): Promise<StoredCredential>;
  refresh(refreshToken: string): Promise<{ access_token: string; refresh_token: string }>;
  // T39 #69 D6: 后端以 Bearer 鉴权换签，必须携带当前 access token。
  switchTenant(input: { tenantId: string; refreshToken: string; accessToken: string }): Promise<{ credential: StoredCredential; tenant?: Record<string, unknown> | null }>;
}

/** Declared here for Task 4, which owns persistence and one-time callback consumption. */
export interface PendingOidcStore {
  savePending(input: PendingOidc): Promise<void>;
  loadPending(): Promise<PendingOidc | undefined>;
  consumePending(): Promise<PendingOidc | undefined>;
  clearPending(): Promise<void>;
}

/** Receives the access token; never exposes it upward. */
export type AuthorizedTransport = (
  input: { method: string; path: string; headers?: Record<string, string>; body?: unknown; signal?: AbortSignal },
  accessToken: string,
) => Promise<unknown>;

/**
 * Authorized SSE read channel. Contract:
 * - a pre-stream 401 rejects (ApiError, status 401) with no chunks emitted;
 * - normal end resolves;
 * - transport resolution failure (per-origin factory returns undefined, fail closed) makes the
 *   runtime reject with 'RUNTIME_STREAM_UNAVAILABLE' — the surface is already authorized, only
 *   the stream channel is absent (REST surfaces remain usable).
 */
export type AuthorizedStreamTransport = (
  input: RuntimeAuthorizedRequest,
  accessToken: string,
  onChunk: (chunk: string) => void,
) => Promise<void>;

export interface PendingOidc {
  deploymentOrigin: string;
  state: string;
  codeVerifier: string;
  redirectUri: string;
}

/** Declared here for Task 4's native browser/deep-link adapter. */
export interface OidcBrowserPort {
  open(authorizationUrl: string): Promise<string>;
}

/** Declared here for Task 4's lifecycle-aware OIDC recovery. */
export interface AppLifecyclePort {
  subscribe(listener: (state: 'active' | 'background') => void): () => void;
}

/** `remoteFor` receives a normalized HTTPS origin, binding transport and credential scope. */
export interface MobileRuntimePorts {
  credentialStore: CredentialStore;
  deploymentStore?: DeploymentStore;
  /** 已登记 Deployment 清单；Runtime 在每次鉴权成功时 upsert 当前实例。 */
  deploymentRegistry?: DeploymentRegistry;
  remoteFor(deployment: string): RuntimeRemote;
  clientVersion: number;
  pendingOidcStore?: PendingOidcStore;
  oidcBrowser?: OidcBrowserPort;
  lifecycle?: AppLifecyclePort;
  /** Scoped Vault Module; the Runtime revokes its scopes on every scope change. Optional so T01-only compositions stay valid. */
  scopedVault?: ScopedVault;
  /** T03: Resource Shelf bindings. When present the Runtime opens one shelf per authorized scope and closes it on every scope change. */
  resourceShelf?: { remoteFor(origin: string): ResourceRemote };
  /** Native platform entropy hook. Omit only where Web Crypto is available. */
  randomBytes?: (size: number) => Uint8Array;
  /** Authorized channel for child modules (Task Office &c.); omitted = fail closed. */
  authorizedTransport?: (deploymentOrigin: string) => AuthorizedTransport;
  /** Authorized SSE channel for child modules; same token discipline as authorizedTransport. May return undefined when the platform has no streaming fetch (fail closed). */
  authorizedStream?: (deploymentOrigin: string) => AuthorizedStreamTransport | undefined;
}
