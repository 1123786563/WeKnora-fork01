import type { ScopeLease } from './types.ts';

/** Package-private scope identity carried by a Runtime-minted lease. Never exported from index.ts. */
export interface LeaseScope {
  deploymentOrigin: string;
  userId: string;
  tenantId: string;
}

/** Runtime-minted, revocable lease. `asScopeLease()` hands out the opaque public view. */
export class RuntimeScopeLease {
  readonly scope: LeaseScope;
  #active = true;
  constructor(scope: LeaseScope) { this.scope = scope; }
  get active(): boolean { return this.#active; }
  revoke(): void { this.#active = false; }
  asScopeLease(): ScopeLease { return this as unknown as ScopeLease; }
}

/** Scope identity extraction for mobile-core Modules (Scoped Vault). Works for revoked leases too — revoke needs the scope to erase. */
export function leaseScopeOf(lease: ScopeLease | undefined): LeaseScope | undefined {
  return lease instanceof RuntimeScopeLease ? lease.scope : undefined;
}

/** Lease validity check; ScopedStore re-validates on every access. */
export function leaseActive(lease: ScopeLease | undefined): boolean {
  return lease instanceof RuntimeScopeLease && lease.active;
}
