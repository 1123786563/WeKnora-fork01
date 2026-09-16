// Pure data helpers for the embed entry chat face, mirroring the Vue embed
// pipeline:
// - history mapping: useEmbedChatSession.ts getmsgList/handleMsgList renders
//   stored session rows (role/content, plus knowledge_references on answers).
// - references extraction: useChatStreamHandler.ts extractKnowledgeReferences
//   (L156-164): data.knowledge_references || data.data.references ||
//   data.data.knowledge_references, arrays only.
// - suggested questions: EmbedChatCore.vue fetchSuggestedQuestions reads
//   res.data.questions and sends item.question on click.
// - headline: docInfo.vue headerText branches (docs / web / mixed / total).
import type { Locale } from '@weknora/i18n/runtime';
import { embedText } from './messages.ts';

export interface EmbedChatMessage {
  role: 'user' | 'assistant';
  content: string;
  references?: EmbedReference[];
}

export interface EmbedReference extends Record<string, unknown> {
  content?: unknown;
  knowledge_title?: unknown;
  chunk_type?: unknown;
  url?: unknown;
}

function asRows(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value) ? (value as Record<string, unknown>[]) : [];
}

export function isWebSearchReference(ref: EmbedReference): boolean {
  return ref.chunk_type === 'web_search';
}

export function referenceUrl(ref: EmbedReference): string {
  const url = typeof ref.url === 'string' ? ref.url.trim() : '';
  return /^https?:\/\//i.test(url) ? url : '';
}

export function referenceTitle(ref: EmbedReference): string {
  if (typeof ref.knowledge_title === 'string' && ref.knowledge_title.trim()) return ref.knowledge_title.trim();
  if (typeof ref.title === 'string' && ref.title.trim()) return ref.title.trim();
  return referenceUrl(ref);
}

export function referenceContent(ref: EmbedReference): string {
  return typeof ref.content === 'string' ? ref.content : '';
}

/** Map stored session rows (GET /messages/:id/load) into chat face entries. */
export function mapHistoryMessages(rows: Record<string, unknown>[]): EmbedChatMessage[] {
  const mapped: EmbedChatMessage[] = [];
  for (const row of rows) {
    const role = row.role === 'user' || row.role === 'assistant' ? row.role : null;
    if (!role) continue;
    const rawContent = row.content;
    const content = typeof rawContent === 'string' ? rawContent : rawContent == null ? '' : String(rawContent);
    if (role === 'user' && !content) continue;
    const references = row.knowledge_references;
    mapped.push({
      role,
      content,
      ...(role === 'assistant' && Array.isArray(references) ? { references } : {}),
    });
  }
  return mapped;
}

/** Pull references off a chat SSE event exactly like extractKnowledgeReferences. */
export function extractStreamReferences(event: unknown): EmbedReference[] {
  if (!event || typeof event !== 'object') return [];
  const data = event as Record<string, unknown>;
  const payload = data.data && typeof data.data === 'object' ? (data.data as Record<string, unknown>) : undefined;
  const refs = data.knowledge_references ?? payload?.references ?? payload?.knowledge_references;
  return Array.isArray(refs) ? (refs as EmbedReference[]) : [];
}

/** Normalize GET /suggested-questions rows into clickable question strings. */
export function normalizeSuggestedQuestions(rows: unknown): string[] {
  return asRows(rows)
    .map((row) => (typeof row.question === 'string' ? row.question.trim() : ''))
    .filter((question) => question.length > 0);
}

/** docInfo.vue headerText branch selection shared with the React face.
 * Docs are counted as document GROUPS keyed by
 * knowledge_id || knowledge_title || id (docInfo.vue groupedKnowledgeRefs),
 * not as raw chunk entries. */
export function referenceHeadline(references: EmbedReference[], locale: Locale): string {
  const docs = references.filter((ref) => !isWebSearchReference(ref));
  const web = references.filter((ref) => isWebSearchReference(ref));
  const docCount = docGroupCount(docs);
  if (docCount > 0 && web.length > 0) {
    return embedText(locale, 'referencesDocAndWebCount', { docCount, webCount: web.length });
  }
  if (docCount > 0) return embedText(locale, 'referencesDocCount', { count: docCount });
  if (web.length > 0) return embedText(locale, 'referencesWebCount', { count: web.length });
  return embedText(locale, 'referencesTitle', { count: references.length });
}

/** docInfo.vue groupedKnowledgeRefs grouping key: one group per document. */
function docGroupCount(docs: EmbedReference[]): number {
  const seen = new Set<string>();
  for (const ref of docs) {
    const id = typeof ref.knowledge_id === 'string' ? ref.knowledge_id : '';
    const title = typeof ref.knowledge_title === 'string' ? ref.knowledge_title : '';
    seen.add(id || title || (typeof ref.id === 'string' ? ref.id : JSON.stringify(ref)));
  }
  return seen.size;
}
