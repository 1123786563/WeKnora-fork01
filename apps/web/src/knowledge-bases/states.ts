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

export interface KnowledgeBaseListItem extends KnowledgeBase {
  permission?: string;
  isMine?: boolean;
  creator_id?: string;
  creator_name?: string;
  is_pinned?: boolean;
  pinned_at?: string;
}

export interface SharedKnowledgeBaseItem {
  knowledge_base: KnowledgeBaseListItem;
  permission?: string;
  share_id?: string;
  shared_at?: string;
  org_name?: string;
  is_mine?: boolean;
}

const permissionRank: Record<string, number> = { viewer: 1, editor: 2, admin: 3, owner: 4 };

/** Merge the Vue all-scope view without duplicate cards or privilege loss. */
export function mergeKnowledgeBaseScopes(
  owned: KnowledgeBaseListItem[],
  shared: SharedKnowledgeBaseItem[],
  userId?: string,
): KnowledgeBaseListItem[] {
  const byId = new Map<string, KnowledgeBaseListItem>();
  for (const item of owned) byId.set(item.id, { ...item, isMine: true });
  for (const entry of shared) {
    const item = entry.knowledge_base;
    if (!item?.id || byId.has(item.id)) continue;
    byId.set(item.id, {
      ...item,
      isMine: item.creator_id && userId ? item.creator_id === userId : false,
      ...(entry.permission ? { permission: entry.permission } : {}),
      ...(entry.share_id ? { share_id: entry.share_id } : {}),
      ...(entry.shared_at ? { shared_at: entry.shared_at } : {}),
      ...(entry.org_name ? { org_name: entry.org_name } : {}),
    });
  }
  return [...byId.values()].sort((a, b) => {
    if (Boolean(a.is_pinned) !== Boolean(b.is_pinned)) return a.is_pinned ? -1 : 1;
    if (a.is_pinned && b.is_pinned) {
      const pinDelta = Date.parse(b.pinned_at ?? '') - Date.parse(a.pinned_at ?? '');
      if (Number.isFinite(pinDelta) && pinDelta !== 0) return pinDelta;
    }
    if (Boolean(a.isMine) !== Boolean(b.isMine)) return a.isMine ? -1 : 1;
    const accessDelta = (permissionRank[b.permission ?? ''] ?? 0) - (permissionRank[a.permission ?? ''] ?? 0);
    return accessDelta || a.name.localeCompare(b.name);
  });
}

/** Client-side filtering used by the Vue list when a query is already loaded. */
export function filterKnowledgeBases(items: KnowledgeBaseListItem[], query: string): KnowledgeBaseListItem[] {
  const needle = query.trim().toLocaleLowerCase();
  if (!needle) return items;
  return items.filter((item) => [item.name, item.description, item.creator_name, item.type]
    .some((value) => typeof value === 'string' && value.toLocaleLowerCase().includes(needle)));
}

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

export interface KnowledgeBaseDetailPermission extends KnowledgeBasePermission {
  canDownload: boolean;
  canMutateDocuments: boolean;
}

/** Permission is evaluated from the effective share, not merely tenant role. */
export function describeKnowledgeBaseDetailPermission(input: {
  permission?: string;
  isOwner?: boolean;
  tenantRole?: string;
  viaShare?: boolean;
}): KnowledgeBaseDetailPermission {
  const permission = input.permission?.toLowerCase();
  const owner = !input.viaShare && (input.isOwner === true || input.tenantRole === 'admin');
  const canEdit = owner || permission === 'owner' || permission === 'admin' || permission === 'editor';
  const canShare = owner || permission === 'owner' || permission === 'admin';
  const canView = input.viaShare ? Boolean(permission) : input.isOwner === true || Boolean(input.tenantRole);
  const canDownload = canView && (input.tenantRole === 'contributor' || input.tenantRole === 'admin' || input.tenantRole === 'owner')
    && (!input.viaShare || permission === 'owner' || permission === 'admin' || permission === 'editor');
  return { canView, canEdit, canShare, canDownload, canMutateDocuments: canEdit && (input.viaShare || owner || input.tenantRole === 'contributor' || input.tenantRole === 'admin') };
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
