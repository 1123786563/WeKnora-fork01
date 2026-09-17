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
  /** Stored row id — required to load per-message follow-up suggestions. */
  id?: string;
  is_completed?: boolean;
  /** Ready follow-up set for this answer (Vue message.suggestionSet). */
  suggestions?: EmbedReadySuggestions | null;
  /** Vue message.suggestionsDismissed — hides the follow-up card. */
  suggestionsDismissed?: boolean;
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
      ...(typeof row.id === 'string' && row.id ? { id: row.id } : {}),
      ...(row.is_completed === true ? { is_completed: true } : {}),
      ...(role === 'assistant' && Array.isArray(references) ? { references } : {}),
    });
  }
  return mapped;
}

// ---------------------------------------------------------------------------
// Scroll-paged history (useEmbedChatSession.ts getmsgList)
// ---------------------------------------------------------------------------

/** Vue created_at cursor: batch[0].created_at of the previous (newest-first) page. */
export function historyCursor(rows: Record<string, unknown>[]): string {
  const first = rows[0];
  return first == null ? '' : String(first.created_at ?? '');
}

export interface HistoryPage {
  messages: EmbedChatMessage[];
  cursor: string;
  hasMore: boolean;
}

/** One getmsgList pass: prepend the batch (already newest-first), advance the
 * cursor, and stop on empty batch / short batch / unchanged cursor exactly
 * like useEmbedChatSession.getmsgList (useChatStreamHandler unshifts the
 * processed batch in order, so the batch order is preserved). */
export function appendHistoryPage(
  current: EmbedChatMessage[],
  rows: Record<string, unknown>[],
  prevCursor: string,
  limit: number,
): HistoryPage {
  if (!rows.length) {
    // Empty batch: also covers the brand-new-session initial load (Vue comment
    // at useEmbedChatSession.ts L203-208).
    return { messages: current, cursor: prevCursor, hasMore: false };
  }
  const nextCursor = historyCursor(rows);
  if (prevCursor && nextCursor === prevCursor) {
    return { messages: current, cursor: prevCursor, hasMore: false };
  }
  const messages = [...mapHistoryMessages(rows), ...current];
  return { messages, cursor: nextCursor, hasMore: rows.length >= limit };
}

/** Vue onChatScrollTop guard: at the top, not already loading, pages remain. */
export function shouldTriggerHistoryLoad(scrollTop: number, loadingOlder: boolean, hasMore: boolean): boolean {
  return scrollTop <= 0 && !loadingOlder && hasMore;
}

/** handleMsgList scroll restore: keep the viewport anchored on the prepended
 * rows via scrollTop = newScrollHeight - oldScrollHeight. */
export function scrollOffsetAfterPrepend(previousHeight: number, nextHeight: number): number {
  return nextHeight - previousHeight;
}

// ---------------------------------------------------------------------------
// Per-message follow-up suggestions (EmbedChatCore.vue loadFollowUpSuggestions)
// ---------------------------------------------------------------------------

export interface EmbedSuggestionItem {
  id: string;
  text: string;
}

export interface EmbedReadySuggestions {
  id: string;
  allowRegenerate: boolean;
  questions: EmbedSuggestionItem[];
}

/** Vue `message.suggestionSet = set?.status === 'ready' ? set : null` plus the
 * non-empty question filter used by the FollowUpSuggestions list. */
export function toReadySuggestions(value: unknown): EmbedReadySuggestions | null {
  if (!value || typeof value !== 'object') return null;
  const set = value as Record<string, unknown>;
  if (set.status !== 'ready') return null;
  const id = typeof set.id === 'string' ? set.id : '';
  if (!id) return null;
  const questions: EmbedSuggestionItem[] = [];
  if (Array.isArray(set.questions)) {
    for (const item of set.questions) {
      if (!item || typeof item !== 'object') continue;
      const row = item as Record<string, unknown>;
      const text = typeof row.text === 'string' ? row.text.trim() : '';
      const itemId = typeof row.id === 'string' ? row.id : '';
      if (!text || !itemId) continue;
      questions.push({ id: itemId, text });
    }
  }
  return { id, allowRegenerate: set.allow_regenerate === true, questions };
}

/** Vue loadFollowUpSuggestions message key: String(message.id || message.assistant_message_id || ''). */
export function followUpMessageId(row: Record<string, unknown>): string {
  if (typeof row.id === 'string' && row.id) return row.id;
  if (typeof row.assistant_message_id === 'string') return row.assistant_message_id;
  return '';
}

export interface EmbedSuggestionAttribution {
  suggestionSetId: string;
  questionId: string;
}

/** useEmbedChatSession.sendMsg attaches the pending attribution picked up from
 * the follow-up click to the outgoing chat request body. */
export function suggestionAttributionBody<T extends Record<string, unknown>>(
  body: T,
  attribution: EmbedSuggestionAttribution | null,
): T & { suggestion_attribution?: { suggestion_set_id: string; question_id: string } } {
  if (!attribution) return body;
  return {
    ...body,
    suggestion_attribution: {
      suggestion_set_id: attribution.suggestionSetId,
      question_id: attribution.questionId,
    },
  };
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

// ---------------------------------------------------------------------------
// Citation pills (frontend/src/utils/citationMarkdown.ts preprocessCitationTags
// + resolveCitationChunkId; rendered as inline pills in EmbedBotMessage and
// resolved to a popover via the embed chunk API).
// ---------------------------------------------------------------------------

export type CitationSegment =
  | { type: 'text'; text: string }
  | { type: 'web'; url: string; title: string; domain: string }
  | { type: 'kb'; doc: string; chunkId: string };

const WEB_TAG_RE = /<web\b([^>]*?)\s*\/?>/gi;
const KB_TAG_RE = /<kb\b([^>]*?)\s*\/?>/gi;
const TAG_ATTR_RE = /([\w-]+)\s*=\s*"([^"]*)"/g;
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function parseTagAttributes(attrString: string): Record<string, string> {
  const attributes: Record<string, string> = {};
  TAG_ATTR_RE.lastIndex = 0;
  let match: RegExpExecArray | null;
  while ((match = TAG_ATTR_RE.exec(attrString)) !== null) attributes[match[1]] = match[2];
  return attributes;
}

function citationDomain(url: string): string {
  try {
    const host = new URL(url).hostname;
    const parts = host.split('.');
    return parts.length >= 2 ? parts.slice(-2).join('.') : host || url;
  } catch {
    return url;
  }
}

function docTitlesMatch(a: string, b: string): boolean {
  const na = a.trim().toLowerCase();
  const nb = b.trim().toLowerCase();
  if (!na || !nb) return false;
  return na === nb || na.includes(nb) || nb.includes(na);
}

/** resolveCitationChunkId: map model context indexes (n, DOC-n, FAQ-n) and doc
 * titles onto the real chunk id from the retrieval references. An empty raw id
 * yields '' so the tag is dropped, exactly like the Vue guard. */
function resolveCitationChunkId(
  rawChunkId: string,
  attrs: { doc?: string },
  refs: Record<string, unknown>[],
): string {
  const raw = rawChunkId.trim();
  if (!raw || UUID_RE.test(raw)) return raw;
  const list = refs.filter((r) => r.chunk_type !== 'web_search');
  if (!list.length) return raw;

  const doc = (attrs.doc || '').trim();
  if (doc) {
    const byDoc = list.find(
      (r) =>
        docTitlesMatch(doc, typeof r.knowledge_title === 'string' ? r.knowledge_title : '') ||
        docTitlesMatch(doc, typeof r.knowledge_filename === 'string' ? r.knowledge_filename : ''),
    );
    if (typeof byDoc?.id === 'string' && byDoc.id) return byDoc.id;
  }

  const faqMatch = raw.match(/^FAQ-(\d+)$/i);
  if (faqMatch) {
    const hit = list.filter((r) => r.chunk_type === 'faq')[parseInt(faqMatch[1], 10) - 1];
    if (hit && typeof hit.id === 'string' && hit.id) return hit.id;
  }
  const docMatch = raw.match(/^DOC-(\d+)$/i);
  if (docMatch) {
    const hit = list.filter((r) => r.chunk_type !== 'faq')[parseInt(docMatch[1], 10) - 1];
    if (hit && typeof hit.id === 'string' && hit.id) return hit.id;
  }

  const num = parseInt(raw, 10);
  if (!Number.isNaN(num) && String(num) === raw) {
    const byPos = list[num - 1];
    if (byPos && typeof byPos.id === 'string' && byPos.id) return byPos.id;
    const byChunkIndex = list.find((r) => r.chunk_index === num || r.chunk_index === num - 1);
    if (byChunkIndex && typeof byChunkIndex.id === 'string' && byChunkIndex.id) return byChunkIndex.id;
  }
  return raw;
}

/** Split answer content into plain text and citation pill segments. Mirrors
 * preprocessCitationTags: <web url title/> becomes a link pill and
 * <kb doc chunk_id/> a popover pill resolved against the references. */
export function parseCitationSegments(
  content: string,
  refs: Record<string, unknown>[] = [],
): CitationSegment[] {
  if (!content) return [];
  const segments: CitationSegment[] = [];
  let last = 0;
  const pattern = /<(web|kb)\b([^>]*?)\s*\/?>/gi;
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(content)) !== null) {
    const attrs = parseTagAttributes(match[2] || '');
    const end = match.index + match[0].length;
    let segment: CitationSegment | null = null;
    if (match[1].toLowerCase() === 'web') {
      const url = attrs.url || '';
      if (url) segment = { type: 'web', url, title: attrs.title || '', domain: citationDomain(url) };
    } else {
      const doc = attrs.doc || '';
      const chunkId = resolveCitationChunkId(attrs.chunk_id || attrs.chunkId || '', { doc }, refs);
      if (doc && chunkId) segment = { type: 'kb', doc, chunkId };
    }
    // Unmatched tags render as '' like the Vue preprocessCitationTags replaces;
    // the surrounding text is kept either way.
    if (match.index > last) segments.push({ type: 'text', text: content.slice(last, match.index) });
    last = end;
    if (segment) segments.push(segment);
  }
  if (last < content.length) segments.push({ type: 'text', text: content.slice(last) });
  // A dropped tag splits the surrounding text into two runs; merge adjacent
  // runs so the face sees the same continuous text as the Vue replace output.
  const merged: CitationSegment[] = [];
  for (const segment of segments) {
    const prev = merged[merged.length - 1];
    if (segment.type === 'text' && prev?.type === 'text') prev.text += segment.text;
    else merged.push(segment);
  }
  return merged.filter((s) => s.type !== 'text' || s.text);
}
