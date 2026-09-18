import type { WikiPage, WikiPageUpdateInput } from '@weknora/api-client';

type WikiWriteApi = { update: (knowledgeBaseId: string, slug: string, input: WikiPageUpdateInput) => Promise<WikiPage> };
export type WikiSaveCopy = { titleRequired: string; contentRequired: string; conflict: string; saveFailed: string };
export type WikiSaveState = { status: 'saved'; page: WikiPage } | { status: 'conflict' | 'error'; message: string };

export function wikiRevertCopy(
  translate: (key: string, values?: Record<string, string | number>) => string,
  kind: 'confirm' | 'failed' | 'success',
  version: number,
): string {
  if (kind === 'confirm') return translate('wikiBrowser.revertConfirm', { ver: version });
  if (kind === 'success') return translate('wikiBrowser.revertSuccess', { ver: version });
  return translate('wikiBrowser.revertFailed');
}

export function applyWikiSearch(draft: string): { draft: string; keyword: string } {
  return { draft, keyword: draft.trim() };
}

/** Vue WikiBrowser's reader state when no page is selected. */
export function wikiReaderEmptyState(
  translate: (key: string) => string,
  hasContentPages: boolean,
): { title: string; description?: string } {
  return hasContentPages
    ? { title: translate('wikiBrowser.selectPageHint'), description: undefined }
    : {
      title: translate('wikiBrowser.emptyTitle'),
      description: translate('wikiBrowser.emptyDesc'),
    };
}

const defaultCopy: WikiSaveCopy = {
  titleRequired: 'Wiki title is required',
  contentRequired: 'Wiki content is required',
  conflict: 'This page changed elsewhere. Reload the latest version before saving.',
  saveFailed: 'Unable to save Wiki page',
};

export function validateWikiPageInput(
  input: { title?: string; content?: string },
  copy: WikiSaveCopy = defaultCopy,
): string | null {
  if (!input.title?.trim()) return copy.titleRequired;
  if (!input.content?.trim()) return copy.contentRequired;
  return null;
}

export async function saveWikiPage(
  api: WikiWriteApi,
  knowledgeBaseId: string,
  slug: string,
  input: WikiPageUpdateInput & { title?: string; content?: string; version: number },
  copy: WikiSaveCopy = defaultCopy,
): Promise<WikiSaveState> {
  const validationError = validateWikiPageInput(input, copy);
  if (validationError) return { status: 'error', message: validationError };
  try {
    return { status: 'saved', page: await api.update(knowledgeBaseId, slug, input) };
  } catch (error) {
    if ((error as { status?: unknown }).status === 409) return { status: 'conflict', message: copy.conflict };
    return { status: 'error', message: error instanceof Error ? error.message : copy.saveFailed };
  }
}

type WikiReadApi = { get: (knowledgeBaseId: string, slug: string) => Promise<WikiPage> };

/**
 * Vue WikiBrowser.vue `overwriteSavePage`: resolve a 409 edit conflict by
 * fetching the server's current page and re-saving the local draft on top of
 * its latest version (last write wins; the losing version stays in revision
 * history, so nothing is destroyed).
 */
export async function overwriteWikiPage(
  api: WikiReadApi & WikiWriteApi,
  knowledgeBaseId: string,
  slug: string,
  draft: { title: string; content: string; summary: string },
  copy: WikiSaveCopy = defaultCopy,
): Promise<WikiSaveState> {
  const validationError = validateWikiPageInput(draft, copy);
  if (validationError) return { status: 'error', message: validationError };
  try {
    const latest = await api.get(knowledgeBaseId, slug);
    return {
      status: 'saved',
      page: await api.update(knowledgeBaseId, slug, { ...draft, version: latest.version }),
    };
  } catch {
    // Vue surfaces the localized editSaveFailed copy here regardless of the
    // underlying error (the raw error is only logged).
    return { status: 'error', message: copy.saveFailed };
  }
}

export function wikiSaveState(error: unknown): Exclude<WikiSaveState, { status: 'saved' }> {
  return { status: 'error', message: error instanceof Error ? error.message : 'Unable to save Wiki page' };
}
