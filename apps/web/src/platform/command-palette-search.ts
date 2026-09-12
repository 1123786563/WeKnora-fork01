// Real chunk/message/KB/agent/session search wiring for the global command
// palette (R011/N003, slice S03 fix round). Mirrors the exact endpoints and
// grouping/matching rules of Vue's authoritative
// frontend/src/components/GlobalCommandPalette/useSearch.ts:
//
//   • Chunk/file search  → POST /api/v1/knowledge-search, scoped to every KB
//     the caller can see (internal/handler/session/qa.go SearchKnowledge),
//     wrapped as client.knowledgeBases.search().
//   • Message/session Q&A search → POST /api/v1/messages/search
//     (internal/handler/message.go SearchMessages), already wrapped as
//     client.settings.chatHistory.search() — reused here, not duplicated.
//   • KB name / agent name matches → client-side filters over the KB list
//     (client.knowledgeBases.list()) and the agent list
//     (client.configuration.agents.list()), exactly as Vue does (no
//     dedicated backend search endpoint exists for either).
//   • Session-by-title matches → GET /api/v1/sessions?keyword=... (real
//     server-side fuzzy title search — internal/handler/session/handler.go),
//     wrapped as client.sessions.list({keyword}). Vue instead filters a
//     client-cached sidebar list; the backend keyword param is a strictly
//     better, still-existing, still-real substitute for the same behavior.
//
// Every network call here swallows backend errors into an empty result
// (mirrors Vue's `.catch(() => [])` / `.catch(() => ({items: [], total: 0}))`)
// so one failing search source never blocks the others from rendering.

import type { KnowledgeChunkSearchHit, WeKnoraClient } from '@weknora/api-client';

export interface CmdkKb {
  id: string;
  name: string;
}

export interface CmdkAgent {
  id: string;
  name: string;
  description: string;
}

export interface CmdkSessionSummary {
  id: string;
  title: string;
}

export interface CmdkFileGroup {
  knowledgeId: string;
  kbId: string;
  title: string;
  kbName: string;
  chunks: KnowledgeChunkSearchHit[];
}

export interface CmdkMessageItem {
  requestId: string;
  sessionId: string;
  sessionTitle: string;
  queryContent: string;
  answerContent: string;
  score: number;
  matchType: string;
  createdAt: string;
}

export interface CmdkMessageGroup {
  sessionId: string;
  sessionTitle: string;
  items: CmdkMessageItem[];
}

// ─── Pure grouping/matching (no network) ───

/** Groups chunk hits by knowledgeId into per-file result groups (mirrors Vue's fmap). */
export function groupChunksByKnowledge(
  hits: readonly KnowledgeChunkSearchHit[],
  kbNameLookup: (kbId: string) => string,
): CmdkFileGroup[] {
  const groups = new Map<string, CmdkFileGroup>();
  for (const hit of hits) {
    const knowledgeId = hit.knowledgeId || 'unknown';
    let group = groups.get(knowledgeId);
    if (!group) {
      group = {
        knowledgeId,
        kbId: hit.knowledgeBaseId,
        title: hit.knowledgeTitle || hit.knowledgeFilename || knowledgeId,
        kbName: kbNameLookup(hit.knowledgeBaseId),
        chunks: [],
      };
      groups.set(knowledgeId, group);
    }
    group.chunks.push(hit);
  }
  return [...groups.values()];
}

/** Groups message Q&A hits by sessionId into per-session result groups (mirrors Vue's mmap). */
export function groupMessagesBySession(items: readonly CmdkMessageItem[]): CmdkMessageGroup[] {
  const groups = new Map<string, CmdkMessageGroup>();
  for (const item of items) {
    const sessionId = item.sessionId || 'unknown';
    let group = groups.get(sessionId);
    if (!group) {
      group = { sessionId, sessionTitle: item.sessionTitle, items: [] };
      groups.set(sessionId, group);
    }
    group.items.push(item);
  }
  return [...groups.values()];
}

/** Case-insensitive substring match over KB names, capped at 4 (mirrors Vue kbMatches). */
export function matchKnowledgeBasesByName(kbs: readonly CmdkKb[], query: string, limit = 4): CmdkKb[] {
  const q = query.trim().toLowerCase();
  if (!q) return [];
  return kbs.filter((kb) => kb.name.toLowerCase().includes(q)).slice(0, limit);
}

/** Case-insensitive name-or-description match over agents, capped at 5 (mirrors Vue agentMatches). */
export function matchAgentsByName(agents: readonly CmdkAgent[], query: string, limit = 5): CmdkAgent[] {
  const q = query.trim().toLowerCase();
  if (!q) return [];
  return agents
    .filter((agent) => agent.name.toLowerCase().includes(q) || agent.description.toLowerCase().includes(q))
    .slice(0, limit);
}

/**
 * Case-insensitive title match over sessions, excluding any session already
 * surfaced by a message-group hit (mirrors Vue's sessionMatches de-dup: the
 * message group is strictly more informative, so it wins).
 */
export function matchSessionsByTitle(
  sessions: readonly CmdkSessionSummary[],
  query: string,
  excludeSessionIds: ReadonlySet<string>,
  limit = 5,
): CmdkSessionSummary[] {
  const q = query.trim().toLowerCase();
  if (!q) return [];
  return sessions
    .filter((session) => session.title.toLowerCase().includes(q))
    .filter((session) => !excludeSessionIds.has(session.id))
    .slice(0, limit);
}

// ─── Message search response parsing ───
//
// client.settings.chatHistory.search() returns the raw (already
// success-validated) `data` envelope as a loose Record<string, unknown> —
// parse it into the typed shape the palette consumes.

export interface ParsedMessageSearch {
  items: CmdkMessageItem[];
  total: number;
}

function parseMessageItem(value: unknown): CmdkMessageItem | null {
  if (value === null || typeof value !== 'object') return null;
  const row = value as Record<string, unknown>;
  if (typeof row.request_id !== 'string' || typeof row.session_id !== 'string') return null;
  return {
    requestId: row.request_id,
    sessionId: row.session_id,
    sessionTitle: typeof row.session_title === 'string' ? row.session_title : '',
    queryContent: typeof row.query_content === 'string' ? row.query_content : '',
    answerContent: typeof row.answer_content === 'string' ? row.answer_content : '',
    score: typeof row.score === 'number' ? row.score : 0,
    matchType: typeof row.match_type === 'string' ? row.match_type : '',
    createdAt: typeof row.created_at === 'string' ? row.created_at : '',
  };
}

export function parseMessageSearchPayload(payload: Record<string, unknown>): ParsedMessageSearch {
  const rawItems = Array.isArray(payload.items) ? payload.items : [];
  const items = rawItems.map(parseMessageItem).filter((item): item is CmdkMessageItem => item !== null);
  const total = typeof payload.total === 'number' ? payload.total : items.length;
  return { items, total };
}

// ─── Network-backed search (each call swallows backend errors to []/empty) ───

export interface SearchKnowledgeChunksInput {
  query: string;
  knowledgeBaseIds: readonly string[];
}

/** No KB scope means nothing to search — mirrors Vue's `kbIds.length > 0` guard; the backend also 400s on an empty scope. */
export async function searchKnowledgeChunks(
  client: Pick<WeKnoraClient, 'knowledgeBases'>,
  input: SearchKnowledgeChunksInput,
  signal?: AbortSignal,
): Promise<KnowledgeChunkSearchHit[]> {
  if (input.knowledgeBaseIds.length === 0) return [];
  try {
    return await client.knowledgeBases.search({
      query: input.query,
      knowledgeBaseIds: input.knowledgeBaseIds,
      ...(signal ? { signal } : {}),
    });
  } catch {
    return [];
  }
}

export interface SearchMessagesInput {
  query: string;
  limit: number;
}

export async function searchMessagesByQuery(
  client: Pick<WeKnoraClient, 'settings'>,
  input: SearchMessagesInput,
  signal?: AbortSignal,
): Promise<ParsedMessageSearch> {
  try {
    const payload = await client.settings.chatHistory.search({ query: input.query, mode: 'hybrid', limit: input.limit }, signal);
    return parseMessageSearchPayload(payload);
  } catch {
    return { items: [], total: 0 };
  }
}

export interface SearchSessionsInput {
  query: string;
  limit: number;
}

/**
 * GET /api/v1/sessions?keyword=... server-side fuzzy title search
 * (internal/handler/session/handler.go). A real, existing, better substitute
 * for Vue's client-cached sidebar-list filter.
 */
export async function searchSessionsByKeyword(
  client: Pick<WeKnoraClient, 'sessions'>,
  input: SearchSessionsInput,
  signal?: AbortSignal,
): Promise<CmdkSessionSummary[]> {
  try {
    const response = await client.sessions.list({ keyword: input.query, pageSize: input.limit, ...(signal ? { signal } : {}) });
    return response.data.map((session) => ({ id: session.id, title: session.title }));
  } catch {
    return [];
  }
}

// ─── Load-once caches for the KB list and agent list ───
//
// Mirrors Vue's session-level kbsLoadingPromise/agentsLoadingPromise caches:
// fetched at most once per palette lifetime, de-duped across concurrent
// callers, and never left permanently "poisoned" by a transient failure (a
// failed load resolves to [] for that call but leaves the cache unloaded so
// a later retry can still succeed).

export interface KbCache {
  loaded: boolean;
  items: CmdkKb[];
  pending: Promise<CmdkKb[]> | null;
}

export function createKbCache(): KbCache {
  return { loaded: false, items: [], pending: null };
}

export async function ensureKnowledgeBasesLoaded(
  client: Pick<WeKnoraClient, 'knowledgeBases'>,
  cache: KbCache,
): Promise<CmdkKb[]> {
  if (cache.loaded) return cache.items;
  if (cache.pending) return cache.pending;
  cache.pending = (async () => {
    try {
      const response = await client.knowledgeBases.list();
      const items = Array.isArray(response) ? response : ((response as unknown as { data?: unknown }).data ?? []);
      cache.items = Array.isArray(items) ? items.flatMap((kb) => {
        if (kb === null || typeof kb !== 'object') return [];
        const row = kb as { id?: unknown; name?: unknown };
        return typeof row.id === 'string' && typeof row.name === 'string' ? [{ id: row.id, name: row.name }] : [];
      }) : [];
      cache.loaded = true;
    } catch {
      cache.items = [];
      cache.loaded = false;
    } finally {
      cache.pending = null;
    }
    return cache.items;
  })();
  return cache.pending;
}

export interface AgentCache {
  loaded: boolean;
  items: CmdkAgent[];
  pending: Promise<CmdkAgent[]> | null;
}

export function createAgentCache(): AgentCache {
  return { loaded: false, items: [], pending: null };
}

export async function ensureAgentsLoaded(
  client: Pick<WeKnoraClient, 'configuration'>,
  cache: AgentCache,
): Promise<CmdkAgent[]> {
  if (cache.loaded) return cache.items;
  if (cache.pending) return cache.pending;
  cache.pending = (async () => {
    try {
      const list = await client.configuration.agents.list();
      cache.items = list.map((agent) => ({ id: agent.id, name: agent.name, description: typeof agent.description === 'string' ? agent.description : '' }));
      cache.loaded = true;
    } catch {
      cache.items = [];
      cache.loaded = false;
    } finally {
      cache.pending = null;
    }
    return cache.items;
  })();
  return cache.pending;
}
