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

function normalizePage(value: unknown): KnowledgeDocumentPage {
  if (typeof value !== 'object' || value === null) throw new Error('Invalid knowledge document page');
  const row = value as Record<string, unknown>;
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
    return { status: 'success', page: normalizePage(await api.list(knowledgeBaseId, params)) };
  } catch (error) {
    return { status: 'error', message: error instanceof Error ? error.message : copy.failed };
  }
}
