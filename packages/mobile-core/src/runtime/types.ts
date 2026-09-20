import type { ClientGateVerdict } from '@weknora/domain/mobile';

export const scopeLeaseBrand: unique symbol = Symbol('ScopeLease');

/** A runtime-issued capability for dependent mobile modules. */
export interface ScopeLease {
  readonly [scopeLeaseBrand]: undefined;
  isActive(): boolean;
}

export type RuntimeSurface = 'signed-out' | 'restoring' | 'full' | 'upgrade-required';

export interface RuntimeIdentity {
  id: string;
}

export interface RuntimeTenant {
  id: string;
}

/** Presentation-safe state: credentials and internal epochs never escape here. */
export interface RuntimeSnapshot {
  surface: RuntimeSurface;
  deployment?: string;
  identity?: RuntimeIdentity;
  tenant?: RuntimeTenant;
  gate?: ClientGateVerdict;
  scopeLease?: ScopeLease;
}

export interface MobileRuntime {
  boot(deploymentHint?: string): Promise<RuntimeSnapshot>;
  signIn(deployment: string, input: { email: string; password: string }): Promise<RuntimeSnapshot>;
  snapshot(): RuntimeSnapshot;
  subscribe(listener: (snapshot: RuntimeSnapshot) => void): () => void;
  signOut(): Promise<void>;
  dispose(): Promise<void>;
}
