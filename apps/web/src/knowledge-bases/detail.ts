import type { KnowledgeBase } from '@weknora/contracts';

export type KnowledgeBaseDetailState =
  | { status: 'ready'; knowledgeBase: KnowledgeBase }
  | { status: 'empty' }
  | { status: 'forbidden'; message: string; code?: string }
  | { status: 'error'; message: string; code?: string };

export type KnowledgeDocumentPreview = Record<string, unknown> & { id: string };
export type KnowledgeDocumentPreviewState =
  | { status: 'ready'; document: KnowledgeDocumentPreview }
  | { status: 'empty' }
  | { status: 'forbidden'; message: string; code?: string }
  | { status: 'error'; message: string; code?: string };

type Requester = (input: { method: string; path: string; signal?: AbortSignal }) => Promise<unknown>;

function unwrap(value: unknown): unknown {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return value;
  const row = value as Record<string, unknown>;
  return row.data ?? row.knowledge_base ?? value;
}

function errorInfo(error: unknown): { message: string; code?: string } {
  const row = error as { message?: unknown; code?: unknown; response?: { status?: unknown; data?: { error?: { code?: unknown } } } };
  const code = typeof row?.code === 'string' ? row.code : typeof row?.response?.data?.error?.code === 'string' ? row.response.data.error.code : typeof row?.response?.status === 'number' ? String(row.response.status) : undefined;
  return { code, message: typeof row?.message === 'string' ? row.message : 'Unable to load knowledge base' };
}

function isForbidden(code?: string): boolean {
  return code === 'FORBIDDEN' || code === 'TENANT_FORBIDDEN' || code === '403';
}

export async function loadKnowledgeBaseDetail(request: Requester, id: string, signal?: AbortSignal): Promise<KnowledgeBaseDetailState> {
  try {
    const value = unwrap(await request({ method: 'GET', path: `/api/v1/knowledge-bases/${encodeURIComponent(id)}`, ...(signal ? { signal } : {}) }));
    if (!value || typeof value !== 'object' || Array.isArray(value)) return { status: 'empty' };
    const knowledgeBase = value as KnowledgeBase;
    return typeof knowledgeBase.id === 'string' && knowledgeBase.id ? { status: 'ready', knowledgeBase } : { status: 'empty' };
  } catch (error) {
    if (signal?.aborted) throw error;
    const info = errorInfo(error);
    return isForbidden(info.code)
      ? { status: 'forbidden', message: info.message, ...(info.code ? { code: info.code } : {}) }
      : { status: 'error', message: info.message, ...(info.code ? { code: info.code } : {}) };
  }
}

export async function loadKnowledgeDocumentPreview(request: Requester, id: string, signal?: AbortSignal): Promise<KnowledgeDocumentPreviewState> {
  try {
    const value = unwrap(await request({ method: 'GET', path: `/api/v1/knowledge/${encodeURIComponent(id)}`, ...(signal ? { signal } : {}) }));
    if (!value || typeof value !== 'object' || Array.isArray(value)) return { status: 'empty' };
    const document = value as KnowledgeDocumentPreview;
    return typeof document.id === 'string' && document.id ? { status: 'ready', document } : { status: 'empty' };
  } catch (error) {
    if (signal?.aborted) throw error;
    const info = errorInfo(error);
    return isForbidden(info.code)
      ? { status: 'forbidden', message: info.message, ...(info.code ? { code: info.code } : {}) }
      : { status: 'error', message: info.message, ...(info.code ? { code: info.code } : {}) };
  }
}

export function buildKnowledgeDocumentSearchParams(input: {
  keyword?: string;
  page?: number;
  pageSize?: number;
  folderPath?: string;
  recursive?: boolean;
  fileType?: string;
  parseStatus?: string;
  source?: string;
}): Record<string, string | number | boolean> {
  const result: Record<string, string | number | boolean> = {};
  if (input.keyword?.trim()) result.keyword = input.keyword.trim();
  if (input.page !== undefined) result.page = input.page;
  if (input.pageSize !== undefined) result.page_size = input.pageSize;
  if (input.folderPath !== undefined) {
    result.folder_path = input.folderPath;
    if (input.recursive) result.folder_recursive = true;
  }
  if (input.fileType) result.file_type = input.fileType;
  if (input.parseStatus) result.parse_status = input.parseStatus;
  if (input.source) result.source = input.source;
  return result;
}
