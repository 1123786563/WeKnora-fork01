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

export type RuntimeSurface = 'deployment-login' | 'upgrade-required' | 'authorized';
export type RuntimeReason = 'protocol-mismatch' | 'unknown-capability' | 'tenant-required' | 'authentication-required';

/** Presentation-safe tenant switcher option; ids match activateTenant input. */
export interface TenantOption {
  id: string;
  name?: string;
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
  scopeLease(): ScopeLease | undefined;
  /** Atomically switches the Active Tenant: revokes the prior scope, re-issues the credential server-side, re-verifies identity. */
  activateTenant(tenantId: string): Promise<RuntimeSnapshot>;
  signOut(): Promise<void>;
  dispose(): void;
}
