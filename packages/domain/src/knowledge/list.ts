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

export interface KnowledgeBaseListResult<T extends KnowledgeBase = KnowledgeBase> {
  items: T[];
  total: number;
  page: number;
  pageCount: number;
}

function searchableText(item: KnowledgeBase): string {
  return [item.name, item.description, item.creator_name].filter((value): value is string => typeof value === 'string').join(' ').toLocaleLowerCase();
}

function isFavorite(item: KnowledgeBase, favoriteIds?: ReadonlySet<string>): boolean {
  return item.is_favorite === true || favoriteIds?.has(item.id) === true;
}

export function filterKnowledgeBases<T extends KnowledgeBase>(items: readonly T[], filter: KnowledgeBaseListFilter = {}): KnowledgeBaseListResult<T> {
  const query = filter.query?.trim().toLocaleLowerCase() ?? '';
  const type = filter.type ?? 'all';
  const creator = filter.creator ?? 'all';
  const filtered = items.filter((item) => {
    if (query && !searchableText(item).includes(query)) return false;
    if (type !== 'all' && (item.type ?? 'document') !== type) return false;
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
