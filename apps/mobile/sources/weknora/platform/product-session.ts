import { createScopeController, type ScopeHandle, type ScopeInput } from '@weknora/domain/scope';

export interface ProductIdentity {
  origin: string;
  userId: string | null;
  tenantId: string | null;
}

export interface ProductClientTeardown {
  stopVoice(): void | Promise<void>;
  disconnectRemote(): void | Promise<void>;
}

export async function closeProductClient(teardown: ProductClientTeardown): Promise<void> {
  await Promise.allSettled([teardown.stopVoice(), teardown.disconnectRemote()]);
}

export interface ProductScope {
  switchTo(next: ProductIdentity): void;
  capture(): { generation: number; signal: AbortSignal };
  accept(generation: number): boolean;
  logout(): void;
  identity(): ProductIdentity;
  subscribe(listener: (identity: ProductIdentity) => void): () => void;
  registerLifecycle(close: () => void | Promise<void>): () => void;
}

function identityOf(handle: ScopeHandle): ProductIdentity {
  const { origin, userId, tenantId } = handle.scope;
  return { origin, userId, tenantId };
}

/**
 * Owns the lifetime of product requests. Every identity transition aborts
 * requests captured under the previous generation, so a late response cannot
 * update a newly selected account or workspace.
 */
export function createProductScope(initial: ProductIdentity): ProductScope {
  const controller = createScopeController(initial satisfies ScopeInput);
  const listeners = new Set<(identity: ProductIdentity) => void>();
  const lifecycles = new Set<() => void | Promise<void>>();

  function advance(next: ProductIdentity): void {
    const handle = controller.switchScope(next.origin, next.userId, next.tenantId);
    const identity = identityOf(handle);
    for (const close of lifecycles) {
      // Lifecycle cleanup is deliberately best-effort. It closes client-side
      // subscriptions; server-side executions remain durable and untouched.
      void Promise.resolve().then(close).catch(() => undefined);
    }
    for (const listener of listeners) listener(identity);
  }

  return {
    switchTo: advance,
    capture() {
      const handle = controller.current();
      return { generation: handle.scope.generation, signal: handle.signal };
    },
    accept(generation) {
      return controller.isCurrent(generation);
    },
    logout() {
      const current = controller.current().scope;
      advance({ origin: current.origin, userId: null, tenantId: null });
    },
    identity() {
      return identityOf(controller.current());
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    registerLifecycle(close) {
      lifecycles.add(close);
      return () => lifecycles.delete(close);
    },
  };
}
