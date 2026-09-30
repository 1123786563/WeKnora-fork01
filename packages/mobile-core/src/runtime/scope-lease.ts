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
  #revocationListeners = new Set<() => void>();
  constructor(scope: LeaseScope) { this.scope = scope; }
  get active(): boolean { return this.#active; }
  revoke(): void {
    if (!this.#active) return;
    this.#active = false;
    for (const listener of [...this.#revocationListeners]) {
      try { listener(); } catch { /* one scoped consumer cannot block revocation for others */ }
    }
    this.#revocationListeners.clear();
  }
  onRevoke(listener: () => void): () => void {
    if (!this.#active) { listener(); return () => undefined; }
    this.#revocationListeners.add(listener);
    return () => { this.#revocationListeners.delete(listener); };
  }
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
