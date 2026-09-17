export interface ScopeIdentity { origin: string; userId: string | null; tenantId: string | null }
export interface ScopeStamp extends ScopeIdentity { generation: number }
export function scopeKey(scope: ScopeIdentity): string {
  return JSON.stringify([scope.origin.replace(/\/+$/, ''), scope.userId, scope.tenantId]);
}
/** Owns cancellation and stale-response rejection, not server authorization. */
export class ScopeGuard {
  private value: ScopeStamp;
  private controllers = new Set<AbortController>();
  constructor(identity: ScopeIdentity) { this.value = { ...identity, generation: 0 }; }
  capture(): Readonly<ScopeStamp> { return Object.freeze({ ...this.value }); }
  isCurrent(stamp: ScopeStamp): boolean { return stamp.generation === this.value.generation && scopeKey(stamp) === scopeKey(this.value); }
  commit(stamp: ScopeStamp, apply: () => void): boolean { if (!this.isCurrent(stamp)) return false; apply(); return true; }
  controller(): AbortController { const c = new AbortController(); this.controllers.add(c); return c; }
  release(controller: AbortController): void { this.controllers.delete(controller); }
  abortAll(): void { for (const c of this.controllers) c.abort(); this.controllers.clear(); }
  switchTo(identity: ScopeIdentity): void { this.abortAll(); this.value = { ...identity, generation: this.value.generation + 1 }; }
  invalidate(): void { this.switchTo(this.value); }
}
