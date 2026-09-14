export interface MobileWorkspace {
  id: number;
  name: string;
  role: string;
}

export interface WorkspaceSelectionStore {
  getItemAsync(key: string): Promise<string | null>;
  setItemAsync(key: string, value: string): Promise<void>;
  deleteItemAsync(key: string): Promise<void>;
}

export function shouldLoadWorkspaceRoute(input: {
  hydrating: boolean;
  credentialKind: 'anonymous' | 'bearer' | 'embed';
}): boolean {
  return !input.hydrating && input.credentialKind === 'bearer';
}

export function createSessionEpoch() {
  let value = 0;
  return {
    current: () => value,
    invalidate: () => { value += 1; return value; },
    isCurrent: (expected: number) => expected === value,
  };
}

/** Serialize platform writes so a late operation cannot finish after a newer one. */
export function createLatestAsyncWriter<T>(write: (value: T) => Promise<void>) {
  let sequence = 0;
  let tail: Promise<void> = Promise.resolve();
  return {
    write(value: T): Promise<void> {
      const current = ++sequence;
      const task = tail.then(async () => {
        if (current !== sequence) return;
        await write(value);
      });
      tail = task.catch(() => undefined);
      return task;
    },
  };
}

export function shouldHydrateWorkspaceMemberships(input: {
  hydrating: boolean;
  credentialKind: 'anonymous' | 'bearer' | 'embed';
  workspaceCount: number;
}): boolean {
  return !input.hydrating && input.credentialKind === 'bearer' && input.workspaceCount === 0;
}

export function shouldRefreshMobileSession(input: {
  transitionCount: number;
  credentialKind: 'anonymous' | 'bearer' | 'embed';
  hasRefreshToken: boolean;
}): boolean {
  return input.transitionCount === 0 && input.credentialKind === 'bearer' && input.hasRefreshToken;
}

/** Clear every in-memory scope when an authenticated refresh is rejected. */
export function resetMobileSessionState(input: {
  updateCredential: (value: { kind: 'anonymous' }) => void;
  updateUserId: (value: string | null) => void;
  updateTenantId: (value: string | null) => void;
  setWorkspaces: (value: MobileWorkspace[]) => void;
  setCanCreateTenant: (value: boolean) => void;
}): void {
  input.updateCredential({ kind: 'anonymous' });
  input.updateUserId(null);
  input.updateTenantId(null);
  input.setWorkspaces([]);
  input.setCanCreateTenant(false);
}

export function createSingleFlight<T>(operation: () => Promise<T>): () => Promise<T> {
  let pending: Promise<T> | null = null;
  return () => {
    if (pending) return pending;
    pending = operation().finally(() => { pending = null; });
    return pending;
  };
}

export function createWorkspaceSelectionAdapter(store: WorkspaceSelectionStore) {
  const key = 'weknora.mobile.selected-workspace';
  return {
    async read(): Promise<number | null> {
      return toWorkspaceId(await store.getItemAsync(key));
    },
    async write(value: number | null): Promise<void> {
      if (value === null) {
        await store.deleteItemAsync(key);
        return;
      }
      const id = toWorkspaceId(value);
      if (id === null) throw new Error('workspace id must be a positive safe integer');
      await store.setItemAsync(key, String(id));
    },
  };
}

export function toWorkspaceId(value: unknown): number | null {
  const numeric = typeof value === 'number' ? value : typeof value === 'string' && value.trim() !== '' ? Number(value) : NaN;
  return Number.isSafeInteger(numeric) && numeric > 0 ? numeric : null;
}

export function parseMobileWorkspaces(values: readonly unknown[] | undefined): MobileWorkspace[] {
  if (!values) return [];
  return values.flatMap((value) => {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) return [];
    const item = value as Record<string, unknown>;
    const id = toWorkspaceId(item.tenant_id ?? item.tenantId ?? item.id);
    const role = typeof item.role === 'string' && item.role.trim() ? item.role : null;
    if (id === null || role === null) return [];
    const name = typeof item.tenant_name === 'string' && item.tenant_name.trim()
      ? item.tenant_name
      : typeof item.tenantName === 'string' && item.tenantName.trim()
        ? item.tenantName
        : typeof item.name === 'string' && item.name.trim() ? item.name : `Workspace ${id}`;
    return [{ id, name, role }];
  });
}
