import type { KnowledgeBase } from '@weknora/contracts';

export type KnowledgeBaseListViewState =
  | { status: 'loading' }
  | { status: 'success'; items: KnowledgeBase[] }
  | { status: 'error'; code?: string; message: string };

export type KnowledgeBaseListBranch =
  | { kind: 'loading' }
  | { kind: 'empty' }
  | { kind: 'ready'; items: KnowledgeBase[] }
  | { kind: 'forbidden'; message: string }
  | { kind: 'error'; message: string };

export function describeKnowledgeBaseList(state: KnowledgeBaseListViewState): KnowledgeBaseListBranch {
  if (state.status === 'loading') return { kind: 'loading' };
  if (state.status === 'success') return state.items.length === 0 ? { kind: 'empty' } : { kind: 'ready', items: state.items };
  return state.code === 'FORBIDDEN' || state.code === 'TENANT_FORBIDDEN'
    ? { kind: 'forbidden', message: state.message }
    : { kind: 'error', message: state.message };
}

export interface KnowledgeBasePermission {
  canView: boolean;
  canEdit: boolean;
  canShare: boolean;
}

export function describeKnowledgeBasePermission(permission: string | undefined): KnowledgeBasePermission {
  if (permission === 'owner' || permission === 'admin') return { canView: true, canEdit: true, canShare: true };
  if (permission === 'editor') return { canView: true, canEdit: true, canShare: false };
  if (permission === 'viewer') return { canView: true, canEdit: false, canShare: false };
  return { canView: false, canEdit: false, canShare: false };
}

export type KnowledgeBaseSaveResult<T> =
  | { status: 'success'; value: T }
  | { status: 'error'; message: string };

export function saveKnowledgeBase<Input, Output>(
  operation: (input: Input) => Promise<Output>,
): (input: Input) => Promise<KnowledgeBaseSaveResult<Output>> {
  let pending: Promise<KnowledgeBaseSaveResult<Output>> | undefined;
  return (input): Promise<KnowledgeBaseSaveResult<Output>> => {
    if (pending) return pending;
    const current = operation(input)
      .then((value) => ({ status: 'success', value }) as const)
      .catch((error: unknown) => ({ status: 'error' as const, message: error instanceof Error ? error.message : 'Unable to save knowledge base' }))
      .finally(() => { pending = undefined; });
    pending = current;
    return current;
  };
}

export type KnowledgeBaseActivityRow = Record<string, unknown>;
export type KnowledgeBaseActivityState =
  | { status: 'empty'; rows: KnowledgeBaseActivityRow[] }
  | { status: 'ready'; rows: KnowledgeBaseActivityRow[] }
  | { status: 'error'; rows: KnowledgeBaseActivityRow[]; message: string };

function activityRows(value: unknown): KnowledgeBaseActivityRow[] {
  if (Array.isArray(value)) return value.filter((row): row is KnowledgeBaseActivityRow => Boolean(row) && typeof row === 'object');
  if (value && typeof value === 'object') {
    const record = value as { data?: unknown; items?: unknown };
    return activityRows(record.data ?? record.items);
  }
  return [];
}

export async function loadKnowledgeBaseActivity(
  request: () => Promise<unknown>,
): Promise<KnowledgeBaseActivityState> {
  try {
    const rows = activityRows(await request());
    return rows.length === 0 ? { status: 'empty', rows } : { status: 'ready', rows };
  } catch (error: unknown) {
    return { status: 'error', rows: [], message: error instanceof Error ? error.message : 'Unable to load activity' };
  }
}
