export interface RequestScope {
  origin: string;
  userId: string | null;
  tenantId: string | null;
  generation: number;
}

export type ScopeInput = Omit<RequestScope, 'generation'>;

export interface ScopeHandle {
  scope: RequestScope;
  signal: AbortSignal;
}

export interface ScopeController {
  current(): ScopeHandle;
  switchScope(origin: string, userId: string | null, tenantId: string | null): ScopeHandle;
  invalidate(): void;
  logout(): void;
  isCurrent(scopeOrGeneration: RequestScope | number): boolean;
}

export function createScopeController(initial: ScopeInput = { origin: '', userId: null, tenantId: null }): ScopeController {
  let scope: RequestScope = { ...initial, generation: 0 };
  let abortController = new AbortController();

  function current(): ScopeHandle {
    return { scope, signal: abortController.signal };
  }

  function advance(nextScope: ScopeInput): ScopeHandle {
    abortController.abort();
    abortController = new AbortController();
    scope = { ...nextScope, generation: scope.generation + 1 };
    return current();
  }

  function switchScope(origin: string, userId: string | null, tenantId: string | null): ScopeHandle {
    return advance({ origin, userId, tenantId });
  }

  function invalidate(): void {
    advance({ origin: scope.origin, userId: scope.userId, tenantId: scope.tenantId });
  }

  function isCurrent(scopeOrGeneration: RequestScope | number): boolean {
    if (typeof scopeOrGeneration === 'number') return scope.generation === scopeOrGeneration;
    return (
      scope.generation === scopeOrGeneration.generation &&
      scope.origin === scopeOrGeneration.origin &&
      scope.userId === scopeOrGeneration.userId &&
      scope.tenantId === scopeOrGeneration.tenantId
    );
  }

  return { current, switchScope, invalidate, logout: invalidate, isCurrent };
}
