// Pure tag helpers for the documents page, ported from the Vue baseline
// frontend/src/views/knowledge/KnowledgeBase.vue + its dialogs
// (BatchTagDialog.vue / TagEditDialog.vue). No React, no i18n.
import type { KnowledgeDocument, KnowledgeTag } from '@weknora/api-client';

const TAG_EST_WIDTH = 82;
const TAG_OVERFLOW_MIN = 32;

/** Vue useTagChipsOverflow.computeLimit parity. */
export function computeTagVisibleLimit(width: number, total: number): number {
  if (total <= 0) return 99;
  const maxFit = Math.floor((width - TAG_OVERFLOW_MIN) / TAG_EST_WIDTH);
  const limit = Math.max(1, Math.min(maxFit, total));
  return limit >= total ? 99 : limit;
}

/** Vue KnowledgeCard.tags (DocumentCardView.vue L35): {id,name,color?}[]. */
export interface DocumentTagRef {
  id: string;
  name?: string;
}

/**
 * Read a document's embedded tags defensively. The shared contract leaves
 * tags untyped ([key: string]: unknown), but the backend list response embeds
 * the same {id,name} rows the Vue card view consumes.
 */
export function documentTags(document: KnowledgeDocument | undefined): DocumentTagRef[] {
  const raw = (document as { tags?: unknown } | undefined)?.tags;
  if (!Array.isArray(raw)) return [];
  const tags: DocumentTagRef[] = [];
  for (const item of raw) {
    if (typeof item === 'object' && item !== null) {
      const row = item as { id?: unknown; name?: unknown };
      if (typeof row.id === 'string' || typeof row.id === 'number') {
        tags.push({ id: String(row.id), name: typeof row.name === 'string' ? row.name : undefined });
      }
    }
  }
  return tags;
}

/**
 * Vue batchTagPreSelectedIds (KnowledgeBase.vue L445-460): the tag ids common
 * to every selected document, seeding the batch tag dialog selection.
 */
export function commonTagIds(documents: readonly KnowledgeDocument[]): string[] {
  let common: Set<string> | null = null;
  for (const document of documents) {
    const ids = documentTags(document).map((tag) => tag.id);
    if (common === null) {
      common = new Set(ids);
      continue;
    }
    for (const id of Array.from(common)) {
      if (!ids.includes(id)) common.delete(id);
    }
  }
  return common ? Array.from(common) : [];
}

/**
 * Vue BatchTagDialog availableTagsList (L145-152): tags not already selected,
 * narrowed by a case-insensitive name search.
 */
export function filterTagOptions(
  tags: readonly KnowledgeTag[],
  selectedIds: readonly string[],
  query: string,
): KnowledgeTag[] {
  const selected = new Set(selectedIds);
  const needle = query.trim().toLowerCase();
  return tags.filter((tag) => {
    if (selected.has(tag.id)) return false;
    if (needle && !(tag.name || '').toLowerCase().includes(needle)) return false;
    return true;
  });
}

/**
 * Vue handleAddNewTag adds an existing tag to the set; it never removes one.
 * This differs from a chip click, which deliberately toggles membership.
 */
export function selectTagId(selectedIds: readonly string[], tagId: string): string[] {
  return selectedIds.includes(tagId) ? [...selectedIds] : [...selectedIds, tagId];
}

/** Vue displays the backend error, with common.operationFailed as its fallback. */
export function tagCreateFailureMessage(error: unknown, fallback: string): string {
  if (typeof error === 'object' && error !== null && 'message' in error) {
    const message = (error as { message?: unknown }).message;
    if (typeof message === 'string' && message.trim()) return message;
  }
  return fallback;
}

/** Vue filterParams (KnowledgeBase.vue L671): tag_ids joins with ',' or drops. */
export function joinTagIds(ids: readonly string[]): string | undefined {
  return ids.length > 0 ? ids.join(',') : undefined;
}

/** Vue activeTagFilterLabel (KnowledgeBase.vue L708-719), split from i18n. */
export type TagFilterLabel =
  | { kind: 'all' }
  | { kind: 'placeholder' }
  | { kind: 'single'; id: string }
  | { kind: 'multi' };

export function tagFilterLabel(selectedIds: readonly string[], cleared: boolean): TagFilterLabel {
  if (selectedIds.length === 0) return cleared ? { kind: 'placeholder' } : { kind: 'all' };
  if (selectedIds.length === 1) return { kind: 'single', id: selectedIds[0] };
  return { kind: 'multi' };
}

/**
 * Vue activeTagFilterTitle (KnowledgeBase.vue L721-729): the trigger tooltip
 * lists the visible tag names joined with 、, falling back to the plain title.
 */
export function tagFilterTitle(
  selectedIds: readonly string[],
  nameOf: (id: string) => string | undefined,
  fallbackTitle: string,
): string {
  if (selectedIds.length === 0) return fallbackTitle;
  const names = selectedIds.map(nameOf).filter((name): name is string => Boolean(name));
  return names.length > 0 ? names.join('、') : fallbackTitle;
}

/** Vue tag panel paging (TAG_PAGE_SIZE=50, load-more at L2510-2515). */
export const TAG_PANEL_PAGE_SIZE = 50;

export function paginateTagOptions(tags: readonly KnowledgeTag[], visibleCount: number): KnowledgeTag[] {
  return tags.slice(0, Math.max(1, visibleCount));
}

/** Vue onBatchTagConfirm (KnowledgeBase.vue L2169-2178): one updates row per document. */
export function tagUpdatesFor(
  documents: readonly { id: string }[],
  tagIds: readonly string[],
): Record<string, string[]> {
  const updates: Record<string, string[]> = {};
  for (const document of documents) updates[document.id] = [...tagIds];
  return updates;
}
