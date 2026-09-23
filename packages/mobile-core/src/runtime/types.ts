import type { ResourceShelfHandle } from '../shelf/types.ts';

declare const scopeLeaseBrand: unique symbol;

/** Caller-supplied deployment metadata; Runtime normalizes `origin` before use. */
export interface DeploymentInput {
  origin: string;
  label?: string;
}

export interface Deployment {
  origin: string;
  label: string;
}

/** Opaque, Runtime-minted capability. Its revocation state never escapes mobile-core. */
export interface ScopeLease {
  readonly [scopeLeaseBrand]: never;
}

export type RuntimeSurface = 'deployment-login' | 'upgrade-required' | 'authorized' | 'read-only';
export type RuntimeReason = 'protocol-mismatch' | 'unknown-capability' | 'tenant-required' | 'authentication-required';

/** Presentation-safe tenant switcher option; ids match activateTenant input. */
export interface TenantOption {
  id: string;
  name?: string;
}

/** Authorized channel input for child modules; structurally assignable from api-client ClientRequest. */
export interface RuntimeAuthorizedRequest {
  method: string;
  path: string;
  headers?: Record<string, string>;
  body?: unknown;
  signal?: AbortSignal;
}

/** Presentation-safe state: credentials, protocol details, leases, and epochs never escape here. */
export interface RuntimeSnapshot {
  surface: RuntimeSurface;
  deployment?: Deployment;
  identity?: { userId: string; activeTenantId?: string; tenants?: TenantOption[] };
  reason?: RuntimeReason;
}

export interface MobileRuntime {
  snapshot(): RuntimeSnapshot;
  subscribe(listener: (snapshot: RuntimeSnapshot) => void): () => void;
  boot(deployment?: DeploymentInput): Promise<RuntimeSnapshot>;
  signIn(input: { deployment: DeploymentInput; email: string; password: string }): Promise<RuntimeSnapshot>;
  beginOidc(input: { deployment: DeploymentInput; redirectUri: string }): Promise<void>;
  completeOidc(callbackUrl: string): Promise<RuntimeSnapshot>;
  /** Sends one request through the active deployment with the current credential (refresh-once on 401). Tokens never escape the Runtime. */
  authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>;
  /** Streams one authorized SSE endpoint through the active deployment (refresh-once on a pre-stream 401). */
  authorizedEventStream(input: RuntimeAuthorizedRequest, onChunk: (chunk: string) => void): Promise<void>;
  scopeLease(): ScopeLease | undefined;
  /** Resource Shelf for the active scope. Undefined unless the surface carries a verified session
   *  (authorized or read-only) AND ports.resourceShelf is provided — read-only browsing is served
   *  from the same shelf seam as the authorized surface (authenticate opens it for both). */
  resourceShelf(): ResourceShelfHandle | undefined;
  /** Atomically switches the Active Tenant: revokes the prior scope, re-issues the credential server-side, re-verifies identity. */
  activateTenant(tenantId: string): Promise<RuntimeSnapshot>;
  /** 登记实例清单（presentation-safe）；未提供 registry 端口时返回空数组。 */
  listDeployments(): Promise<Deployment[]>;
  /** 原子切换 Active Deployment：撤销旧 scope，恢复目标实例已存凭据并重新验证身份；未登记、畸形 origin 或无凭据时 fail closed。 */
  switchDeployment(origin: string): Promise<RuntimeSnapshot>;
  /** 移除一个登记实例并清除其凭据；移除活动实例等价于登出。 */
  forgetDeployment(origin: string): Promise<void>;
  signOut(): Promise<void>;
  dispose(): void;
}
