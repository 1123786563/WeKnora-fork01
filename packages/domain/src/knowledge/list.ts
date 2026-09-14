import type { KnowledgeBase } from '@weknora/contracts';

export type KnowledgeBaseCreatorFilter = 'all' | 'mine' | 'others';

export interface KnowledgeBaseListFilter {
  query?: string;
  type?: 'all' | 'document' | 'faq';
  creator?: KnowledgeBaseCreatorFilter;
  currentUserId?: string;
  favoritesOnly?: boolean;
  favoriteIds?: ReadonlySet<string>;
  page?: number;
  pageSize?: number;
}

export interface KnowledgeBaseListResult<T extends Record<string, unknown> & { id: string } = KnowledgeBase> {
  items: T[];
  total: number;
  page: number;
  pageCount: number;
}

function searchableText(item: Record<string, unknown>): string {
  return [item.name, item.description, item.creator_name].filter((value): value is string => typeof value === 'string').join(' ').toLocaleLowerCase();
}

function isFavorite(item: Record<string, unknown>, favoriteIds?: ReadonlySet<string>): boolean {
  return item.is_favorite === true || favoriteIds?.has(String(item.id)) === true;
}

export function filterKnowledgeBases<T extends Record<string, unknown> & { id: string }>(items: readonly T[], filter: KnowledgeBaseListFilter = {}): KnowledgeBaseListResult<T> {
  const query = filter.query?.trim().toLocaleLowerCase() ?? '';
  const type = filter.type ?? 'all';
  const creator = filter.creator ?? 'all';
  const filtered = items.filter((item) => {
    if (query && !searchableText(item).includes(query)) return false;
    if (type !== 'all' && ((item.type as string | undefined) ?? 'document') !== type) return false;
    if (creator === 'mine' && (!filter.currentUserId || item.creator_id !== filter.currentUserId)) return false;
    if (creator === 'others' && (!filter.currentUserId || !item.creator_id || item.creator_id === filter.currentUserId)) return false;
    if (filter.favoritesOnly && !isFavorite(item, filter.favoriteIds)) return false;
    return true;
  });
  const pageSize = Number.isSafeInteger(filter.pageSize) && filter.pageSize! > 0 ? filter.pageSize! : 24;
  const pageCount = filtered.length === 0 ? 0 : Math.ceil(filtered.length / pageSize);
  const requestedPage = Number.isSafeInteger(filter.page) && filter.page! > 0 ? filter.page! : 1;
  const page = pageCount === 0 ? 1 : Math.min(requestedPage, pageCount);
  const start = (page - 1) * pageSize;
  return { items: filtered.slice(start, start + pageSize), total: filtered.length, page, pageCount };
}

// ---- Knowledge-base list card model (ported from frontend/src/views/knowledge) ----

export interface OwnedKnowledgeBase {
  id: string;
  is_pinned?: boolean;
  pinned_at?: string;
  created_at?: string;
  creator_id?: string;
  [key: string]: unknown;
}

export interface SharedKnowledgeBaseLike {
  knowledge_base: {
    id: string;
    knowledge_count?: number;
    chunk_count?: number;
    [key: string]: unknown;
  } | null;
  permission: string;
  shared_at: string;
  share_id: string;
  org_name?: string;
  [key: string]: unknown;
}

export type MergedOwnedKnowledgeBase = OwnedKnowledgeBase & { isMine: true };
export type MergedSharedKnowledgeBase = Record<string, unknown> & {
  id: string;
  isMine: false;
  permission: string;
  shared_at: string;
  share_id: string;
  org_name?: string;
};
export type MergedKnowledgeBase = MergedOwnedKnowledgeBase | MergedSharedKnowledgeBase;

const EDITABLE_PERMS = new Set(['admin', 'editor']);

export function isSharedKbEditable(perm: string | undefined): boolean {
  return !!perm && EDITABLE_PERMS.has(perm);
}

const PERMISSION_RANK: Record<string, number> = { admin: 3, editor: 2, viewer: 1 };

function permissionRank(perm: string | undefined): number {
  return (perm && PERMISSION_RANK[perm]) || 0;
}

function isMyKb(kb: { creator_id?: string }, currentUserId: string | undefined): boolean {
  return !!(kb.creator_id && currentUserId && kb.creator_id === currentUserId);
}

function pinnedTime(kb: OwnedKnowledgeBase): number {
  return kb.pinned_at ? Date.parse(kb.pinned_at) : 0;
}

/**
 * De-duplicated, ordered list for the "all" scope. Ordering: pinned
 * (newest pin first) -> own KBs -> teammate KBs -> shared (editable
 * grants first). Owned rows win over shared duplicates, and repeated
 * shares of one KB collapse to the most-privileged permission so every
 * rendered card key stays unique.
 */
export function mergeAllScopeKnowledgeBases(
  owned: OwnedKnowledgeBase[],
  shared: SharedKnowledgeBaseLike[],
  currentUserId: string | undefined,
): MergedKnowledgeBase[] {
  const result: MergedKnowledgeBase[] = [];

  const pinned: OwnedKnowledgeBase[] = [];
  const ownMine: OwnedKnowledgeBase[] = [];
  const teammateMine: OwnedKnowledgeBase[] = [];
  const ownedIds = new Set<string>();
  for (const kb of owned) {
    ownedIds.add(kb.id);
    if (kb.is_pinned) pinned.push(kb);
    else if (isMyKb(kb, currentUserId)) ownMine.push(kb);
    else teammateMine.push(kb);
  }
  pinned.sort((a, b) => pinnedTime(b) - pinnedTime(a));

  for (const kb of pinned) result.push({ ...kb, isMine: true as const });
  for (const kb of ownMine) result.push({ ...kb, isMine: true as const });
  for (const kb of teammateMine) result.push({ ...kb, isMine: true as const });

  const dedupedShared = new Map<string, SharedKnowledgeBaseLike>();
  for (const entry of shared) {
    const kb = entry?.knowledge_base;
    if (!kb) continue;
    if (ownedIds.has(kb.id)) continue;
    const existing = dedupedShared.get(kb.id);
    if (!existing || permissionRank(entry.permission) > permissionRank(existing.permission)) {
      dedupedShared.set(kb.id, entry);
    }
  }

  const sortedShared = [...dedupedShared.values()].sort((a, b) => {
    const aE = isSharedKbEditable(a.permission) ? 0 : 1;
    const bE = isSharedKbEditable(b.permission) ? 0 : 1;
    return aE - bE;
  });

  for (const sharedEntry of sortedShared) {
    const kb = sharedEntry.knowledge_base!;
    result.push({
      ...kb,
      isMine: false as const,
      permission: sharedEntry.permission,
      shared_at: sharedEntry.shared_at,
      share_id: sharedEntry.share_id,
      org_name: sharedEntry.org_name,
      knowledge_count: kb.knowledge_count,
      chunk_count: kb.chunk_count,
    } as MergedSharedKnowledgeBase);
  }

  return result;
}

export interface KnowledgeBaseViewer {
  userId: string;
  isAdmin?: boolean;
  isContributor?: boolean;
}

/**
 * Gates the destructive per-card items (Edit / Delete / Settings). Mirrors
 * KnowledgeBaseList.vue canManageKBCard: creator match OR admin fallback.
 * Legacy KBs with an empty creator_id rely on the admin gate alone.
 */
export function canManageKBCard(
  kb: { creator_id?: unknown },
  viewer: KnowledgeBaseViewer,
): boolean {
  if (kb.creator_id && viewer.userId && String(kb.creator_id) === viewer.userId) return true;
  return viewer.isAdmin === true;
}

/** Duplicate is contributor-only and never offered for shared KB cards. */
export function canDuplicateKBCard(
  kb: { creator_id?: unknown; isMine?: unknown },
  viewer: KnowledgeBaseViewer,
): boolean {
  return viewer.isContributor === true && kb.isMine !== false;
}

/** Mirrors the Vue isInitialized rule (summary always, embedding for RAG). */
export function isKnowledgeBaseInitialized(kb: {
  summary_model_id?: string;
  embedding_model_id?: string;
  indexing_strategy?: { vector_enabled?: boolean; keyword_enabled?: boolean; wiki_enabled?: boolean } | null;
}): boolean {
  if (!kb.summary_model_id || kb.summary_model_id === '') return false;
  const strategy = kb.indexing_strategy;
  const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled;
  if (needsEmbedding && (!kb.embedding_model_id || kb.embedding_model_id === '')) return false;
  return true;
}
export interface KnowledgeBaseSection<T> {
  key: 'pinned' | 'mine' | 'tenantOthers' | 'sharedEditable' | 'sharedReadonly';
  /** i18n key suffix under knowledgeList.sections.* */
  labelKey: string;
  items: T[];
}

/** Port of Vue KnowledgeBaseList.vue section grouping (lines 97-185, 932-949):
 *  pinned (newest pinned_at first), mine, tenant others, shared editable,
 *  shared readonly. Empty sections are omitted; sharedByMe is not derivable
 *  from the merged rows and is handled by the shared-KB source data. */
export function groupKnowledgeBaseSections<T extends Record<string, unknown> & { id: string }>(
  rows: readonly T[],
  currentUserId: string | undefined,
  options?: { tenantReadonly?: boolean },
): KnowledgeBaseSection<T>[] {
  const buckets: Record<KnowledgeBaseSection<T>['key'], T[]> = {
    pinned: [], mine: [], tenantOthers: [], sharedEditable: [], sharedReadonly: [],
  };
  const pinnedTime = (value: unknown): number => {
    const parsed = Date.parse(typeof value === 'string' ? value : '');
    return Number.isNaN(parsed) ? 0 : parsed;
  };
  for (const row of rows) {
    const isSharedEntry = row.isMine === false;
    if (row.is_pinned === true) { buckets.pinned.push(row); continue; }
    if (isSharedEntry) {
      const permission = typeof row.permission === 'string' ? row.permission : undefined;
      buckets[isSharedKbEditable(permission) ? 'sharedEditable' : 'sharedReadonly'].push(row);
      continue;
    }
    const creator = typeof row.creator_id === 'string' ? row.creator_id : undefined;
    if (currentUserId && creator && creator === currentUserId) buckets.mine.push(row);
    else buckets.tenantOthers.push(row);
  }
  buckets.pinned.sort((a, b) => pinnedTime(b.pinned_at) - pinnedTime(a.pinned_at));
  // Vue tenantSectionLabelKey (KnowledgeBaseList.vue:1083-1086): contributor/
  // viewer lack write access to tenantOthers KBs, so their group header reads
  // "仅查看"; admin/owner get the ownership-toned label instead.
  const labelKeys: Record<string, string> = {
    pinned: 'knowledgeList.sections.pinned',
    mine: 'knowledgeList.sections.mine',
    tenantOthers: options?.tenantReadonly === true
      ? 'knowledgeList.sections.tenantReadonly'
      : 'knowledgeList.sections.tenantOthers',
    sharedEditable: 'knowledgeList.sections.sharedEditable',
    sharedReadonly: 'knowledgeList.sections.sharedReadonly',
  };
  return (Object.keys(buckets) as KnowledgeBaseSection<T>['key'][])
    .filter((key) => buckets[key].length > 0)
    .map((key) => ({ key, labelKey: labelKeys[key], items: buckets[key] }));
}

export type KnowledgeBaseScope = 'all' | 'mine' | 'favorites' | 'recents';

/** Port of Vue KnowledgeBaseList.vue scope filtering: all / mine / favorites /
 *  recents. Favorites and recents are caller-provided id sets (Vue stores them
 *  in per-user localStorage). */
export function filterByScope<T extends { id: string }>(
  rows: readonly T[],
  scope: KnowledgeBaseScope,
  currentUserId: string | undefined,
  favoriteIds: ReadonlySet<string> = new Set(),
  recentIds: ReadonlySet<string> = new Set(),
): T[] {
  switch (scope) {
    case 'all': return [...rows];
    case 'favorites': return rows.filter((r) => favoriteIds.has(r.id));
    case 'recents': return rows.filter((r) => recentIds.has(r.id));
    case 'mine': return rows.filter((r) => {
      const creator = (r as Record<string, unknown>).creator_id;
      return typeof creator === 'string' && creator === currentUserId;
    });
  }
}
