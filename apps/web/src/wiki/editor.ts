import type { WikiPage, WikiPageUpdateInput } from '@weknora/api-client';

type WikiWriteApi = { update: (knowledgeBaseId: string, slug: string, input: WikiPageUpdateInput) => Promise<WikiPage> };
export type WikiSaveState = { status: 'saved'; page: WikiPage } | { status: 'conflict' | 'error'; message: string };

export async function saveWikiPage(
  api: WikiWriteApi,
  knowledgeBaseId: string,
  slug: string,
  input: WikiPageUpdateInput & { title?: string; content?: string; version: number },
): Promise<WikiSaveState> {
  if (!input.title?.trim()) return { status: 'error', message: 'Wiki title is required' };
  if (!input.content?.trim()) return { status: 'error', message: 'Wiki content is required' };
  try {
    return { status: 'saved', page: await api.update(knowledgeBaseId, slug, input) };
  } catch (error) {
    if ((error as { status?: unknown }).status === 409) return { status: 'conflict', message: 'This page changed elsewhere. Reload the latest version before saving.' };
    return { status: 'error', message: error instanceof Error ? error.message : 'Unable to save Wiki page' };
  }
}

export function wikiSaveState(error: unknown): Exclude<WikiSaveState, { status: 'saved' }> {
  return { status: 'error', message: error instanceof Error ? error.message : 'Unable to save Wiki page' };
}
