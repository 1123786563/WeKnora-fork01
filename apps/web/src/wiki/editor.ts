import type { WikiPage, WikiPageUpdateInput } from '@weknora/api-client';

type WikiWriteApi = { update: (knowledgeBaseId: string, slug: string, input: WikiPageUpdateInput) => Promise<WikiPage> };
export type WikiSaveCopy = { titleRequired: string; contentRequired: string; conflict: string; saveFailed: string };
export type WikiSaveState = { status: 'saved'; page: WikiPage } | { status: 'conflict' | 'error'; message: string };

export function applyWikiSearch(draft: string): { draft: string; keyword: string } {
  return { draft, keyword: draft.trim() };
}

const defaultCopy: WikiSaveCopy = {
  titleRequired: 'Wiki title is required',
  contentRequired: 'Wiki content is required',
  conflict: 'This page changed elsewhere. Reload the latest version before saving.',
  saveFailed: 'Unable to save Wiki page',
};

export async function saveWikiPage(
  api: WikiWriteApi,
  knowledgeBaseId: string,
  slug: string,
  input: WikiPageUpdateInput & { title?: string; content?: string; version: number },
  copy: WikiSaveCopy = defaultCopy,
): Promise<WikiSaveState> {
  if (!input.title?.trim()) return { status: 'error', message: copy.titleRequired };
  if (!input.content?.trim()) return { status: 'error', message: copy.contentRequired };
  try {
    return { status: 'saved', page: await api.update(knowledgeBaseId, slug, input) };
  } catch (error) {
    if ((error as { status?: unknown }).status === 409) return { status: 'conflict', message: copy.conflict };
    return { status: 'error', message: error instanceof Error ? error.message : copy.saveFailed };
  }
}

export function wikiSaveState(error: unknown): Exclude<WikiSaveState, { status: 'saved' }> {
  return { status: 'error', message: error instanceof Error ? error.message : 'Unable to save Wiki page' };
}
