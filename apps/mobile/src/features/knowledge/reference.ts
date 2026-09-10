import type { Href } from 'expo-router';

export type KnowledgeReferenceKind = 'wiki' | 'faq';

export function selectWikiReferenceLabel(row: { title: string; slug: string; version: number }): string {
  const title = row.title.trim() || row.slug.trim() || 'Untitled Wiki page';
  return `${title} · v${row.version}`;
}

export function selectFaqReferenceLabel(row: { standard_question: string; is_enabled: boolean; is_recommended: boolean }): string {
  const question = row.standard_question.trim() || 'Untitled FAQ';
  const state = row.is_enabled ? 'enabled' : 'disabled';
  return `${question} · ${state}${row.is_recommended ? ' · recommended' : ''}`;
}

export function referenceRoute(kind: KnowledgeReferenceKind, kbId: string): Href {
  return `/knowledge/${encodeURIComponent(kbId)}/${kind}` as Href;
}
