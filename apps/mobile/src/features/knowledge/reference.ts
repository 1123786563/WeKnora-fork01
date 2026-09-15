import type { Href } from 'expo-router';

export type KnowledgeReferenceKind = 'wiki' | 'faq';

export interface WikiReferenceCopy {
  untitled: string;
  version: (version: number) => string;
}

export function selectWikiReferenceLabel(
  row: { title: string; slug: string; version: number },
  copy: WikiReferenceCopy = { untitled: 'Untitled Wiki page', version: (version) => `v${version}` },
): string {
  const title = row.title.trim() || row.slug.trim() || copy.untitled;
  return `${title} · ${copy.version(row.version)}`;
}

export function selectWikiReferenceEditKey(row: { id: string; slug: string }): string {
  return row.slug;
}

export interface FaqReferenceCopy {
  untitled: string;
  enabled: string;
  disabled: string;
  recommended: string;
}

export function selectFaqReferenceLabel(
  row: { standard_question: string; is_enabled: boolean; is_recommended: boolean },
  copy: FaqReferenceCopy = { untitled: 'Untitled FAQ', enabled: 'enabled', disabled: 'disabled', recommended: 'recommended' },
): string {
  const question = row.standard_question.trim() || copy.untitled;
  const state = row.is_enabled ? copy.enabled : copy.disabled;
  return `${question} · ${state}${row.is_recommended ? ` · ${copy.recommended}` : ''}`;
}

export function selectFaqReferenceEditKey(row: { id: number }): string {
  return String(row.id);
}

export function faqReferenceListParams(query: string): { page: 1; page_size: 100; keyword?: string } {
  const keyword = query.trim();
  return keyword ? { page: 1, page_size: 100, keyword } : { page: 1, page_size: 100 };
}

export function referenceRoute(kind: KnowledgeReferenceKind, kbId: string): Href {
  return `/knowledge/${encodeURIComponent(kbId)}/${kind}` as Href;
}

export function editorRoute(kind: KnowledgeReferenceKind, kbId: string, slug?: string): Href {
  const query = new URLSearchParams({ kind });
  if (slug) query.set('slug', slug);
  return `/knowledge/${encodeURIComponent(kbId)}/editor?${query.toString()}` as Href;
}
