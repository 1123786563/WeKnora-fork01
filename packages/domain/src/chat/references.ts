export type ChatReferenceKind = 'web' | 'document' | 'tool';

export interface ChatReferenceItem {
  key: string;
  kind: ChatReferenceKind;
  title: string;
  content?: string;
  snippet?: string;
  url?: string;
  domain?: string;
  source?: string;
  chunkId?: string;
  chunkIds?: readonly string[];
  knowledgeId?: string;
  knowledgeBaseId?: string;
}

export interface ChatReferenceGroup {
  kind: ChatReferenceKind;
  items: readonly ChatReferenceItem[];
}

type UnknownRecord = Record<string, unknown>;

function record(value: unknown): UnknownRecord | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as UnknownRecord
    : null;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function metadata(value: unknown): UnknownRecord {
  return record(value) ?? {};
}

function cleanPreview(value: unknown, maxLength = 220): string {
  const normalized = text(value)
    // Strip only leading ellipsis runs left over from streamed previews; the
    // original opening text must survive (Vue docInfo renders from char 0).
    .replace(/^…+\s*/, '')
    .replace(/!\[[^\]]*]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]+)]\([^)]*\)/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/\*([^*]+)\*/g, '$1')
    .replace(/(^|\s)#+\s*/g, '$1')
    .replace(/\s+/g, ' ')
    .trim();
  return normalized.length > maxLength ? `${normalized.slice(0, maxLength)}…` : normalized;
}

/** Return a stable, clickable URL only for ordinary web destinations. */
export function normalizeReferenceUrl(value: unknown): string | null {
  const raw = text(value);
  if (!raw) return null;
  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
    parsed.hash = '';
    if (parsed.pathname.length > 1) parsed.pathname = parsed.pathname.replace(/\/+$/, '');
    return parsed.toString();
  } catch {
    return null;
  }
}

function urlDomain(url: string): string {
  return new URL(url).hostname.replace(/^www\./i, '');
}

function referenceId(item: UnknownRecord): string {
  return text(item.id);
}

function chunkIds(item: UnknownRecord): string[] {
  const ids = Array.isArray(item.chunk_ids) ? item.chunk_ids.map(text).filter(Boolean) : [];
  const id = referenceId(item);
  if (id && !ids.includes(id)) ids.push(id);
  return ids;
}

function webItem(item: UnknownRecord): ChatReferenceItem | null {
  const meta = metadata(item.metadata);
  const url = normalizeReferenceUrl(meta.url) ?? normalizeReferenceUrl(item.id);
  if (!url) return null;
  const domain = urlDomain(url);
  const candidateTitle = text(meta.title) || text(item.knowledge_title);
  const title = normalizeReferenceUrl(candidateTitle) || candidateTitle === domain ? domain : candidateTitle || domain;
  const content = text(item.content);
  const snippet = cleanPreview(meta.snippet) || cleanPreview(content);
  return {
    key: `web:${url}`,
    kind: 'web',
    title,
    url,
    domain,
    ...(snippet ? { snippet } : {}),
    ...(content ? { content } : {}),
    ...(referenceId(item) ? { chunkId: referenceId(item) } : {}),
  };
}

interface MutableDocumentReference extends ChatReferenceItem {
  chunkIds: string[];
  contentParts: string[];
}

function documentGroupKey(item: UnknownRecord, index: number): string {
  const knowledgeId = text(item.knowledge_id);
  if (knowledgeId) return knowledgeId;
  const title = text(item.knowledge_title) || text(item.knowledge_filename) || text(item.title);
  const knowledgeBaseId = text(item.knowledge_base_id);
  if (title) return [knowledgeBaseId, title].filter(Boolean).join(':');
  return referenceId(item) || `reference-${index}`;
}

function toolItem(item: UnknownRecord, index: number): ChatReferenceItem {
  const meta = metadata(item.metadata);
  const id = referenceId(item) || `tool-${index}`;
  const content = text(item.content);
  const snippet = cleanPreview(content);
  const source = text(meta.source) || text(meta.tool);
  return {
    key: `tool:${id}`,
    kind: 'tool',
    title: text(item.knowledge_title) || text(meta.title) || 'Tool result',
    chunkId: id,
    ...(source ? { source } : {}),
    ...(snippet ? { snippet } : {}),
    ...(content ? { content } : {}),
  };
}

/**
 * Convert untrusted API/SSE reference payloads into a small, UI-independent model.
 * Invalid entries and unsafe web schemes are omitted; document chunks are grouped.
 */
export function groupChatReferences(values: readonly unknown[] | null | undefined): ChatReferenceGroup[] {
  if (!Array.isArray(values)) return [];

  const web = new Map<string, ChatReferenceItem>();
  const documents = new Map<string, MutableDocumentReference>();
  const tools: ChatReferenceItem[] = [];

  values.forEach((value, index) => {
    const item = record(value);
    if (!item) return;
    const chunkType = text(item.chunk_type);
    if (chunkType === 'web_search') {
      const normalized = webItem(item);
      if (normalized && !web.has(normalized.key)) web.set(normalized.key, normalized);
      return;
    }
    if (chunkType === 'tool_result') {
      tools.push(toolItem(item, index));
      return;
    }

    const groupKey = documentGroupKey(item, index);
    const existing = documents.get(groupKey);
    const content = text(item.content);
    const ids = chunkIds(item);
    if (existing) {
      for (const id of ids) if (!existing.chunkIds.includes(id)) existing.chunkIds.push(id);
      if (content && !existing.contentParts.includes(content)) existing.contentParts.push(content);
      return;
    }

    const knowledgeId = text(item.knowledge_id);
    const knowledgeBaseId = text(item.knowledge_base_id);
    const title = text(item.knowledge_title) || text(item.knowledge_filename) || text(item.title) || knowledgeId || 'Document';
    documents.set(groupKey, {
      key: `document:${groupKey}`,
      kind: 'document',
      title,
      chunkId: referenceId(item) || undefined,
      chunkIds: ids,
      contentParts: content ? [content] : [],
      ...(knowledgeId ? { knowledgeId } : {}),
      ...(knowledgeBaseId ? { knowledgeBaseId } : {}),
    });
  });

  const documentItems = [...documents.values()].map(({ contentParts, ...item }) => {
    const content = contentParts.slice(0, 3).join('\n\n');
    return {
      ...item,
      ...(content ? { content, snippet: cleanPreview(content) } : {}),
    };
  });

  return [
    { kind: 'web' as const, items: [...web.values()] },
    { kind: 'document' as const, items: documentItems },
    { kind: 'tool' as const, items: tools },
  ].filter((group) => group.items.length > 0);
}
