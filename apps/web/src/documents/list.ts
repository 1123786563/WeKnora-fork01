import type { KnowledgeDocument, KnowledgeDocumentListParams } from '@weknora/api-client';

export interface KnowledgeDocumentPage {
  items: KnowledgeDocument[];
  total: number;
  page: number;
  pageSize: number;
}

export type KnowledgeDocumentListState =
  | { status: 'loading' }
  | { status: 'success'; page: KnowledgeDocumentPage }
  | { status: 'error'; message: string };

type DocumentListApi = {
  list: (knowledgeBaseId: string, params?: KnowledgeDocumentListParams) => Promise<unknown>;
};

export interface KnowledgeDocumentListCopy {
  unavailable: string;
  failed: string;
}

function hasActiveDocumentFilter(params: KnowledgeDocumentListParams, keyword: string | undefined): boolean {
  return Boolean(
    keyword ||
    params.tag_ids?.trim() ||
    params.file_type?.trim() ||
    params.parse_status?.trim() ||
    params.source?.trim() ||
    params.start_time?.trim() ||
    params.end_time?.trim(),
  );
}

/**
 * Mirrors Vue KnowledgeBase.vue filterParams: root is an explicit empty folder
 * path, and every active filter widens that path to its complete subtree.
 */
function normalizeListParams(params: KnowledgeDocumentListParams): KnowledgeDocumentListParams {
  const keyword = params.keyword?.trim() || undefined;
  return {
    ...params,
    keyword,
    folder_path: params.folder_path ?? '',
    folder_recursive: hasActiveDocumentFilter(params, keyword),
  };
}

function normalizePage(value: unknown): KnowledgeDocumentPage {
  if (typeof value !== 'object' || value === null) throw new Error('Invalid knowledge document page');
  const row = value as Record<string, unknown>;
  if ('success' in row && row.success !== true) throw new Error('Invalid knowledge document page');
  if (Array.isArray(row.items)) {
    return {
      items: row.items as KnowledgeDocument[],
      total: Number(row.total ?? row.items.length),
      page: Number(row.page ?? 1),
      pageSize: Number(row.pageSize ?? row.page_size ?? 20),
    };
  }
  if (Array.isArray(row.data)) {
    return {
      items: row.data as KnowledgeDocument[],
      total: Number(row.total ?? row.data.length),
      page: Number(row.page ?? 1),
      pageSize: Number(row.page_size ?? 20),
    };
  }
  throw new Error('Invalid knowledge document page');
}

/**
 * Vue getPageSize (KnowledgeBase.vue L810-816): the document list is a
 * viewport-adaptive infinite-scroll surface — floor(innerHeight / 148) * 5
 * items per request, floored at 35, computed once per mount. No paginator.
 */
export function documentListPageSize(viewportHeight: number): number {
  const itemHeight = 148;
  const itemsInView = Math.floor(viewportHeight / itemHeight) * 5;
  return Math.max(35, itemsInView);
}

/**
 * Vue useKnowledgeBase getKnowled append semantics: page 1 replaces the list,
 * page > 1 (scroll load) appends behind the already-loaded slice. A failed
 * scroll load keeps the current success state instead of blanking the list
 * (Vue cardList is untouched when the scroll fetch errors).
 */
export function mergeDocumentListState(
  current: KnowledgeDocumentListState,
  next: KnowledgeDocumentListState,
  page: number,
): KnowledgeDocumentListState {
  if (page <= 1 || current.status !== 'success') return next;
  if (next.status !== 'success') return current;
  const seen = new Set(current.page.items.map((item) => item.id));
  const appended = next.page.items.filter((item) => !seen.has(item.id));
  return {
    status: 'success',
    page: { ...next.page, items: [...current.page.items, ...appended] },
  };
}

export async function loadKnowledgeDocuments(
  client: { knowledge?: { documents?: DocumentListApi }; knowledgeBases?: { documents?: DocumentListApi } },
  knowledgeBaseId: string,
  params: KnowledgeDocumentListParams = {},
  copy: KnowledgeDocumentListCopy = {
    unavailable: 'Document API is unavailable',
    failed: 'Unable to load documents',
  },
): Promise<KnowledgeDocumentListState> {
  const api = client.knowledge?.documents ?? client.knowledgeBases?.documents;
  if (!api) return { status: 'error', message: copy.unavailable };
  try {
    return { status: 'success', page: normalizePage(await api.list(knowledgeBaseId, normalizeListParams(params))) };
  } catch (error) {
    return { status: 'error', message: error instanceof Error ? error.message : copy.failed };
  }
}
