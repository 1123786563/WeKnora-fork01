import type { KnowledgeDocument, KnowledgeDocumentListResponse } from '@weknora/contracts';

export interface KnowledgeParitySnapshot {
  total: number;
  ids: string[];
  statuses: Record<string, string | undefined>;
}

export function selectKnowledgeParity(value: KnowledgeDocumentListResponse): KnowledgeParitySnapshot {
  return {
    total: value.total,
    ids: value.data.map((item) => item.id),
    statuses: Object.fromEntries(value.data.map((item) => [item.id, item.parse_status])),
  };
}

export function selectKnowledgeDocumentLabel(document: KnowledgeDocument): string {
  return document.file_name || document.title || document.id;
}

export function isTerminalKnowledgeStatus(status: string | undefined): boolean {
  return status === 'completed' || status === 'failed' || status === 'cancelled';
}

export function shouldUseRecursiveFolderScope(filters: {
  folderPath?: string;
  keyword?: string;
  tagIds?: readonly string[];
}): boolean {
  return Boolean(
    filters.folderPath &&
    (filters.keyword?.trim() || filters.tagIds?.length),
  );
}
