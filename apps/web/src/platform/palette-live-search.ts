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
      // F4 — Vue useSearch 语义：各源各自落地各自更新（kbMatches/agentMatches
      // 是 computed，客户端名匹配在缓存就绪后即时出现；chunks/messages 各自
      // 响应到达后补齐），不再被单一 Promise.all 闸在最慢远端源上。
      // loading 只由远端搜索（chunks + 非 scoped 的 messages）把关，与 Vue
      // runSearch 的 Promise.all([knowledge, messages]) 收口一致。
      const scoped = scopeKbIds.length > 0;
      let remotePending = scoped ? 1 : 2;
      const settleRemote = () => {
        remotePending -= 1;
        if (remotePending === 0 && seq === seqRef.current) {
          setState((current) => ({ ...current, loading: false }));
        }
      };

      // KB 名匹配：缓存列表就绪即发布（不等任何远端搜索）。
      const kbsPromise = scoped
        ? Promise.resolve([] as CmdkKb[])
        : ensureKnowledgeBasesLoaded(client, kbCacheRef.current).catch(() => [] as CmdkKb[]);
      void kbsPromise.then((kbs) => {
        if (seq !== seqRef.current) return;
        setState((current) => ({
          ...current,
          knowledgeBases: kbs,
          kbMatches: scoped ? [] : matchKnowledgeBasesByName(kbs, trimmed),
        }));
      });

      // 智能体名匹配：智能体列表就绪即发布。
      if (!scoped && agentsEnabled) {
        ensureAgentsLoaded(client, agentCacheRef.current)
          .then((agents) => {
            if (seq !== seqRef.current) return;
            setState((current) => ({ ...current, agentMatches: matchAgentsByName(agents, trimmed) }));
          })
          .catch(() => { /* cache load failure keeps the prior (empty) matches */ });
      }

      // 会话标题匹配：与消息组联动去重（Vue sessionMatches 依赖
      // messageGroups 的 computed），任一侧到达都按最新两侧结果重发布。
      const sessionState = { sessions: [] as CmdkSessionSummary[], messageGroups: [] as CmdkMessageGroup[] };
      const publishSessions = () => {
        if (seq !== seqRef.current) return;
        const covered = new Set(sessionState.messageGroups.map((group) => group.sessionId));
        setState((current) => ({ ...current, sessionMatches: scoped ? [] : matchSessionsByTitle(sessionState.sessions, trimmed, covered) }));
      };

      // 知识块搜索：需要 kbIds（scoped 用锁定范围，否则等 KB 缓存）。
      void (async () => {
        const kbs = await kbsPromise;
        const kbIds = scoped ? [...scopeKbIds] : kbs.map((kb) => kb.id);
        const chunks = await searchKnowledgeChunks(client, { query: trimmed, knowledgeBaseIds: kbIds });
        if (seq !== seqRef.current) return;
        const nameOf = (kbId: string): string => kbs.find((kb) => kb.id === kbId)?.name ?? '';
        setState((current) => ({ ...current, fileGroups: groupChunksByKnowledge(chunks, nameOf), totalChunks: chunks.length }));
        settleRemote();
      })();

      // 消息搜索（scoped 下禁用：loading 计数不含此腿，不额外收口）。
      if (!scoped) {
        void searchMessagesByQuery(client, { query: trimmed, limit: 30 }).then((payload) => {
          if (seq !== seqRef.current) { settleRemote(); return; }
          sessionState.messageGroups = groupMessagesBySession(payload.items);
          setState((current) => ({ ...current, messageGroups: sessionState.messageGroups, totalMessages: payload.total }));
          publishSessions(); // 消息组到达后重新去重会话标题匹配（Vue computed）
          settleRemote();
        });
      }

      // 会话标题搜索（scoped 下禁用）。
      if (!scoped) {
        void searchSessionsByKeyword(client, { query: trimmed, limit: 20 }).then((sessions) => {
          if (seq !== seqRef.current) return;
          sessionState.sessions = sessions;
          publishSessions();
        });
      }
    }, debounceMs);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, enabled, trimmed, scopeSignature, debounceMs, agentsEnabled]);

  return state;
}

// Re-exported so the palette only imports from one module.
export { parseMessageSearchPayload };
