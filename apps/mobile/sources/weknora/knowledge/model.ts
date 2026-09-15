import type { KnowledgeDocument } from '@weknora/api-client';

export type KnowledgePermission = { canView: boolean; canEdit: boolean };
export type KnowledgeViewState = 'loading' | 'forbidden' | 'error' | 'ready';

const encode = (value: string) => encodeURIComponent(value);

export const knowledgePaths = {
  detail: (id: string) => `/api/v1/knowledge/${encode(id)}`,
  preview: (id: string) => `/api/v1/knowledge/${encode(id)}/preview`,
  download: (id: string) => `/api/v1/knowledge/${encode(id)}/download`,
  wikiPages: (kbId: string) => `/api/v1/knowledgebase/${encode(kbId)}/wiki/pages`,
  faqEntries: (kbId: string) => `/api/v1/knowledge-bases/${encode(kbId)}/faq/entries`,
};

export function canKnowledgeAction(action: 'preview' | 'download' | 'wiki-edit' | 'faq-edit', permission: KnowledgePermission): boolean {
  if (!permission.canView) return false;
  return action === 'preview' || action === 'download' ? true : permission.canEdit;
}

export function getKnowledgeViewState(input: { loading: boolean; permission: KnowledgePermission; error?: string | null }): KnowledgeViewState {
  if (input.loading) return 'loading';
  if (!input.permission.canView) return 'forbidden';
  if (input.error) return 'error';
  return 'ready';
}

export function documentTitle(document: Pick<KnowledgeDocument, 'id' | 'title' | 'file_name'>): string {
  return String(document.title || document.file_name || 'Untitled document');
}
