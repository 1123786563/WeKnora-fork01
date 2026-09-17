// R464-A1 — live search orchestration for the React GlobalCommandPalette.
// React port of frontend/src/components/GlobalCommandPalette/useSearch.ts:
//   • 350ms debounce (overridable), non-empty trimmed query;
//   • fan-out: knowledge chunks (POST /api/v1/knowledge-search via
//     client.knowledgeBases.search) + chat messages + session titles, with
//     client-side KB/agent name matching over lazily-cached lists;
//   • a non-empty `scopeKbIds` locks chunk search to those KBs and disables
//     every other source (Vue scoped groupOrder is chunks-only);
//   • stale responses are dropped via a monotonically-increasing sequence id;
//   • every source swallows backend errors into an empty result so one
//     failing endpoint never blocks the others (command-palette-search.ts).
import { useEffect, useMemo, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import {
  createAgentCache,
  createKbCache,
  ensureAgentsLoaded,
  ensureKnowledgeBasesLoaded,
  groupChunksByKnowledge,
  groupMessagesBySession,
  matchAgentsByName,
  matchKnowledgeBasesByName,
  matchSessionsByTitle,
  parseMessageSearchPayload,
  searchKnowledgeChunks,
  searchMessagesByQuery,
  searchSessionsByKeyword,
  type CmdkAgent,
  type CmdkFileGroup,
  type CmdkKb,
  type CmdkMessageGroup,
  type CmdkSessionSummary,
} from './command-palette-search.ts';

/** Everything the palette's live search needs from the API client. */
export type PaletteSearchClient = Pick<WeKnoraClient, 'knowledgeBases' | 'settings' | 'sessions' | 'configuration'>;

export interface PaletteLiveSearchState {
  loading: boolean;
  hasSearched: boolean;
  fileGroups: CmdkFileGroup[];
  messageGroups: CmdkMessageGroup[];
  kbMatches: CmdkKb[];
  agentMatches: CmdkAgent[];
  sessionMatches: CmdkSessionSummary[];
  totalChunks: number;
  totalMessages: number;
  /** All visible KBs — also used to backfill the scope chip's display name. */
  knowledgeBases: CmdkKb[];
}

const EMPTY_STATE: PaletteLiveSearchState = {
  loading: false,
  hasSearched: false,
  fileGroups: [],
  messageGroups: [],
  kbMatches: [],
  agentMatches: [],
  sessionMatches: [],
  totalChunks: 0,
  totalMessages: 0,
  knowledgeBases: [],
};

export interface UsePaletteLiveSearchOptions {
  /** Full API client; null keeps the palette in commands-only mode (S03). */
  client: PaletteSearchClient | null;
  /** Current input text (already controlled by the palette). */
  query: string;
  /** Only search while the palette is open. */
  enabled: boolean;
  /** Locked KB scope; non-empty disables message/session/agent/KB groups. */
  scopeKbIds: readonly string[];
  /** Debounce in ms — Vue default 350; tests shrink it. */
  debounceMs: number;
  /**
   * R465-A2 — Vue useCmdkSearch `agentsEnabled`:
   * deploymentCapabilities.isSupported('agents'). When false the agent list
   * is never fetched and the agent name-match group stays empty. Defaults to
   * true (fail-open like the Vue capability store).
   */
  agentsEnabled?: boolean;
}

export function usePaletteLiveSearch(options: UsePaletteLiveSearchOptions): PaletteLiveSearchState {
  const { client, query, enabled, scopeKbIds, debounceMs, agentsEnabled = true } = options;
  const [state, setState] = useState<PaletteLiveSearchState>(EMPTY_STATE);
  const seqRef = useRef(0);
  const kbCacheRef = useRef(createKbCache());
  const agentCacheRef = useRef(createAgentCache());

  const trimmed = query.trim();
  const scopeSignature = useMemo(() => [...scopeKbIds].sort().join(','), [scopeKbIds]);

  // Vue preloads the KB list on mount so name matches (and the scope chip's
  // display name) are available on the first keystroke. Mirror that lazily:
  // once per palette open, before/independent of any query.
  useEffect(() => {
    if (!client || !enabled) return;
    void ensureKnowledgeBasesLoaded(client, kbCacheRef.current).then((kbs) => {
      setState((current) => (current.knowledgeBases === kbs ? current : { ...current, knowledgeBases: kbs }));
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, enabled]);

  useEffect(() => {
    if (!client) return;
    if (!enabled) {
      // Palette closed — drop results so the next open starts clean (Vue's
      // clearResults() on the open watcher's else branch).
      seqRef.current += 1;
      setState(EMPTY_STATE);
      return;
    }
    if (!trimmed) {
      seqRef.current += 1; // cancel any in-flight request
      setState((current) => (current === EMPTY_STATE ? current : { ...EMPTY_STATE }));
      return;
    }
    const timer = setTimeout(() => {
      const seq = ++seqRef.current;
      setState((current) => ({ ...current, loading: true, hasSearched: true }));
      void (async () => {
        try {
          const scoped = scopeKbIds.length > 0;
          const kbs = scoped ? [] : await ensureKnowledgeBasesLoaded(client, kbCacheRef.current);
          const kbIds = scoped ? [...scopeKbIds] : kbs.map((kb) => kb.id);
          const [chunks, messagePayload, sessions, agents] = await Promise.all([
            searchKnowledgeChunks(client, { query: trimmed, knowledgeBaseIds: kbIds }),
            scoped ? Promise.resolve({ items: [], total: 0 }) : searchMessagesByQuery(client, { query: trimmed, limit: 30 }),
            scoped ? Promise.resolve([] as CmdkSessionSummary[]) : searchSessionsByKeyword(client, { query: trimmed, limit: 20 }),
            scoped || !agentsEnabled ? Promise.resolve([] as CmdkAgent[]) : ensureAgentsLoaded(client, agentCacheRef.current),
          ]);
          if (seq !== seqRef.current) return; // stale response guard

          const nameOf = (kbId: string): string => kbs.find((kb) => kb.id === kbId)?.name ?? '';
          const fileGroups = groupChunksByKnowledge(chunks, nameOf);
          const messageGroups = groupMessagesBySession(messagePayload.items);
          const coveredSessions = new Set(messageGroups.map((group) => group.sessionId));
          setState({
            loading: false,
            hasSearched: true,
            fileGroups,
            messageGroups,
            kbMatches: scoped ? [] : matchKnowledgeBasesByName(kbs, trimmed),
            agentMatches: scoped ? [] : matchAgentsByName(agents, trimmed),
            sessionMatches: scoped ? [] : matchSessionsByTitle(sessions, trimmed, coveredSessions),
            totalChunks: chunks.length,
            totalMessages: messagePayload.total,
            knowledgeBases: kbs,
          });
        } catch {
          if (seq !== seqRef.current) return;
          setState((current) => ({ ...current, loading: false }));
        }
      })();
    }, debounceMs);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, enabled, trimmed, scopeSignature, debounceMs, agentsEnabled]);

  return state;
}

// Re-exported so the palette only imports from one module.
export { parseMessageSearchPayload };
