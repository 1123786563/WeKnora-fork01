import * as React from 'react';
import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { formatMessage, type Locale } from '@weknora/i18n';
// S6 换装（T15 前置）：搜索框离开 packages/ui 旧栈 Input——Vue GlobalCommandPalette.vue:16
// 本就是原生 <input class="cmdk__input">（视觉规则 wk-cmdk-11 = Vue .cmdk__input），
// 直接落原生标签，DOM/焦点/键盘语义与测试锚点不变。
import {
  COMMANDS,
  filterCommands,
  nextSelectedIndex,
  paletteShortcutDigit,
  shortcutDigitFor,
  visibleCommands,
  type CommandDescriptor,
} from './command-palette.ts';
// R484 D17 — the product-tour command re-opens the welcome guide the same way
// Vue commands.ts openNewUserGuide() does (contextualGuides.ts event dispatch).
import { openNewUserGuide } from '@weknora/views';
import './platform-u.css';
import {
  usePaletteLiveSearch,
  type PaletteSearchClient,
} from './palette-live-search.ts';
import type {
  CmdkAgent,
  CmdkFileGroup,
  CmdkKb,
  CmdkMessageGroup,
  CmdkSessionSummary,
} from './command-palette-search.ts';

export interface GlobalCommandPaletteProps {
  /** Whether the dialog is currently shown. Mirrors Vue's `open` store ref. */
  open: boolean;
  /** Query to seed the input with when the palette opens (e.g. from `?cmdk=`). */
  initialQuery: string;
  /** Persisted recent searches, most-recent-first. Empty list hides the group. */
  recentQueries: string[];
  locale: Locale;
  onClose: () => void;
  /** Navigate the app to a command's target path. */
  onNavigate: (path: string) => void;
  /** Called when a search (non-empty query) leads to running a command, so the
   *  caller can persist it to recent queries (mirrors Vue pushRecent()). */
  onSearch: (query: string) => void;
  onClearRecent: () => void;
  /**
   * R464-A1 — live knowledge/content search. When provided (PlatformShell
   * passes the real client), typing fans out to /api/vnowledge-search etc.
   * and renders chunk/message/KB/agent/session groups like Vue. Absent keeps
   * the S03 commands-only palette (and the pre-R464 tests) intact.
   */
  searchClient?: PaletteSearchClient | null;
  /** KB scope chip seed — Vue infers it from the KB detail route on open. */
  initialKbScope?: { id: string; name: string } | null;
  /** Debounce for live search; Vue default 350ms. Overridable for tests. */
  searchDebounceMs?: number;
  /**
   * R465-A2 — deployment-capability gating for the static quick actions
   * (Vue GlobalCommandPalette.vue:262-270 filters open-agents /
   * open-organizations through the deploymentCapabilities store). Defaults
   * to both-visible so callers that have not probed capabilities yet keep
   * the fail-open behaviour.
   */
  access?: { canOpenAgents: boolean; canOpenOrganizations: boolean };
  /**
   * R465-A2 — Vue useCmdkSearch `agentsEnabled`. When false the agent list
   * is never fetched and the agent name-match group stays hidden. Defaults
   * to true (fail-open).
   */
  agentsEnabled?: boolean;
  /**
   * R465-A2 — Vue empty-state askAi(): record the query as a recent search,
   * close the palette and start a new chat seeded with the query. When the
   * prop is absent the palette falls back to navigating to
   * /platform/creatChat?q=… (the chat composer does not consume the prefill
   * yet — see the R465 report), so shell-owned startChat wiring can replace
   * it later without touching this component again.
   */
  onAskAi?: (query: string, kbIds: string[]) => void;
  /**
   * R465-A2 — retrieval-settings drawer content (Vue lines 153-157 host
   * RetrievalSettings inside a 420px t-drawer layered over the palette).
   * Provide a ReactNode to render both the drawer and its two triggers (the
   * input-row settings icon and the empty-state "Adjust retrieval" button);
   * absent keeps both hidden.
   */
  retrievalSettings?: ReactNode;
}

/** Per-group display caps — keep the palette compact (Vue CHUNK_LIMIT/MSG_LIMIT). */
const CHUNK_LIMIT = 5;
const MSG_LIMIT = 4;

interface FlatChunkItem { file: CmdkFileGroup; chunk: CmdkFileGroup['chunks'][number] }
interface FlatMessageItem { group: CmdkMessageGroup; msg: CmdkMessageGroup['items'][number] }

function flatChunkItemsOf(fileGroups: readonly CmdkFileGroup[]): FlatChunkItem[] {
  const out: FlatChunkItem[] = [];
  for (const file of fileGroups) {
    for (const chunk of file.chunks) {
      out.push({ file, chunk });
      if (out.length >= CHUNK_LIMIT) return out;
    }
  }
  return out;
}

function flatMessageItemsOf(messageGroups: readonly CmdkMessageGroup[]): FlatMessageItem[] {
  const out: FlatMessageItem[] = [];
  for (const group of messageGroups) {
    for (const msg of group.items) {
      out.push({ group, msg });
      if (out.length >= MSG_LIMIT) return out;
    }
  }
  return out;
}

/** Split text around case-insensitive query matches (safe <mark> highlight). */
function highlightSegments(text: string, query: string): Array<{ text: string; hit: boolean }> {
  const needle = query.trim();
  if (!needle || needle.length > 200) return [{ text, hit: false }];
  const lower = text.toLowerCase();
  const needleLower = needle.toLowerCase();
  const out: Array<{ text: string; hit: boolean }> = [];
  let index = 0;
  while (index < text.length) {
    const found = lower.indexOf(needleLower, index);
    if (found === -1) {
      out.push({ text: text.slice(index), hit: false });
      break;
    }
    if (found > index) out.push({ text: text.slice(index, found), hit: false });
    out.push({ text: text.slice(found, found + needle.length), hit: true });
    index = found + needle.length;
  }
  return out.filter((part) => part.text.length > 0);
}

function Highlighted({ text, query }: { text: string; query: string }): ReactNode {
  return (
    <>
      {highlightSegments(text, query).map((part, index) =>
        part.hit ? <mark key={index} className="wk-cmdk-1">{part.text}</mark> : <span key={index}>{part.text}</span>,
      )}
    </>
  );
}

/**
 * React port of frontend/src/components/GlobalCommandPalette.vue (R011/N003,
 * S03 + R464 live search). Quick actions, recents and command matching as
 * before; typing with a `searchClient` now fans out to knowledge chunks,
 * chat messages, KB/agent names and session titles exactly like Vue's
 * useCmdkSearch (350ms debounce, chunks-first group order, KB scope chip).
 */
export function GlobalCommandPalette(props: GlobalCommandPaletteProps): ReactNode {
  const {
    open, initialQuery, recentQueries, locale, onClose, onNavigate, onSearch, onClearRecent,
    searchClient = null, initialKbScope = null, searchDebounceMs = 350,
    access = { canOpenAgents: true, canOpenOrganizations: true },
    agentsEnabled = true,
    onAskAi,
    retrievalSettings = null,
  } = props;
  const t = (key: string): string => formatMessage(locale, key);
  const [query, setQuery] = useState(initialQuery);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [kbScope, setKbScope] = useState<{ id: string; name: string } | null>(initialKbScope);
  const [retrievalDrawerVisible, setRetrievalDrawerVisible] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const resultsRef = useRef<HTMLDivElement | null>(null);

  const live = usePaletteLiveSearch({
    client: searchClient,
    query,
    enabled: open,
    scopeKbIds: useMemo(() => (kbScope ? [kbScope.id] : []), [kbScope]),
    debounceMs: searchDebounceMs,
    agentsEnabled,
  });

  useEffect(() => {
    if (!open) return;
    setQuery(initialQuery);
    setSelectedIndex(0);
    setKbScope(initialKbScope);
    setRetrievalDrawerVisible(false);
    const raf = requestAnimationFrame(() => inputRef.current?.focus());
    return () => cancelAnimationFrame(raf);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initialKbScope]);

  // Backfill the scope chip's display name once the KB list resolves (Vue's
  // knowledgeBases watcher: falls back from the raw id to the real name).
  useEffect(() => {
    if (!kbScope || kbScope.name !== kbScope.id) return;
    const match = live.knowledgeBases.find((kb) => kb.id === kbScope.id);
    if (match) setKbScope({ id: match.id, name: match.name });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [live.knowledgeBases]);

  const trimmed = query.trim();
  // R465-A2 — deployment capability gating (Vue allCommands filter):
  // open-agents / open-organizations come and go with the deployment
  // capabilities; everything else is always visible.
  const baseCommands = useMemo(() => visibleCommands(COMMANDS, access), [access]);
  const items: CommandDescriptor[] = useMemo(
    () => (trimmed ? filterCommands(baseCommands, trimmed, t) : [...baseCommands]),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [trimmed, locale, baseCommands],
  );
  const recentCount = trimmed ? 0 : recentQueries.length;
  const scoped = kbScope !== null;
  // Vue groupOrder (GlobalCommandPalette.vue:279-289): scoped → chunks only;
  // otherwise chunks → messages → kbs → agents → sessions → commands.
  const showCommandsGroup = !trimmed || !scoped;
  const flatChunkItems = useMemo(() => flatChunkItemsOf(live.fileGroups), [live.fileGroups]);
  const flatMessageItems = useMemo(() => flatMessageItemsOf(live.messageGroups), [live.messageGroups]);
  const kbMatches: CmdkKb[] = scoped ? [] : live.kbMatches;
  const agentMatches: CmdkAgent[] = scoped ? [] : live.agentMatches;
  const sessionMatches: CmdkSessionSummary[] = scoped ? [] : live.sessionMatches;
  const commandCount = showCommandsGroup ? items.length : 0;

  const flatCount = recentCount + flatChunkItems.length + flatMessageItems.length
    + kbMatches.length + agentMatches.length + sessionMatches.length + commandCount;

  const scrollToSelected = (index: number): void => {
    const el = resultsRef.current?.querySelector<HTMLElement>(`[data-cmdk-index="${index}"]`);
    if (el && typeof el.scrollIntoView === 'function') el.scrollIntoView({ block: 'nearest' });
  };

  if (!open) return null;

  /** Record the query into recents, close, navigate — the shared result "open". */
  const openResult = (path: string): void => {
    if (trimmed) onSearch(trimmed);
    onClose();
    onNavigate(path);
  };

  /**
   * R465-A2 — Vue askAi() (GlobalCommandPalette.vue:459-464): record the
   * query as a recent search, close the palette and start a new chat with
   * the query pre-filled. The shell can inject its own startChat via
   * `onAskAi`; the default navigates to /platform/creatChat?q=… so the
   * prefill survives the SPA navigation (the React composer does not
   * consume it yet — see the R465-A2 report for the cross-domain blocker).
   */
  const askAi = (): void => {
    if (!trimmed) return;
    onSearch(trimmed);
    onClose();
    // R467-A2 — Vue startChat(query, kbIds): the palette's active KB scope
    // rides along as ?kbIds= so the new chat starts scoped; the custom
    // onAskAi receives the ids for shell-owned startChat wiring.
    const kbIds = kbScope ? [kbScope.id] : [];
    if (onAskAi) onAskAi(trimmed, kbIds);
    else {
      const params = new URLSearchParams({ q: trimmed });
      if (kbIds.length > 0) params.set('kbIds', kbIds.join(','));
      onNavigate(`/platform/creatChat?${params.toString()}`);
    }
  };

  const runFlat = (index: number, fromKeyboard = false): void => {
    if (index < 0 || index >= flatCount) {
      if (fromKeyboard) return;
      return;
    }
    if (index < recentCount) {
      const value = recentQueries[index];
      if (value !== undefined) {
        setQuery(value);
        setSelectedIndex(0);
        inputRef.current?.focus();
      }
      return;
    }
    let i = index - recentCount;
    if (i < flatChunkItems.length) {
      const item = flatChunkItems[i]!;
      const suffix = item.file.kbId ? `?knowledge_id=${encodeURIComponent(item.file.knowledgeId)}` : '';
      openResult(`/platform/knowledge-bases/${encodeURIComponent(item.file.kbId)}${suffix}`);
      return;
    }
    i -= flatChunkItems.length;
    if (i < flatMessageItems.length) {
      const item = flatMessageItems[i]!;
      if (item.group.sessionId) openResult(`/platform/chat/${encodeURIComponent(item.group.sessionId)}`);
      return;
    }
    i -= flatMessageItems.length;
    if (i < kbMatches.length) {
      openResult(`/platform/knowledge-bases/${encodeURIComponent(kbMatches[i]!.id)}`);
      return;
    }
    i -= kbMatches.length;
    if (i < agentMatches.length) {
      openResult(`/platform/creatChat?agent_id=${encodeURIComponent(agentMatches[i]!.id)}`);
      return;
    }
    i -= agentMatches.length;
    if (i < sessionMatches.length) {
      const session = sessionMatches[i]!;
      if (session.id) openResult(`/platform/chat/${encodeURIComponent(session.id)}`);
      return;
    }
    i -= sessionMatches.length;
    const command = items[i];
    if (command) {
      if (trimmed) onSearch(trimmed);
      onClose();
      // R484 D17 — Vue commands.ts:86-93 open-product-tour: the tour command
      // re-opens the welcome guide (weknora:open-new-user-guide event) instead
      // of navigating; PlatformShell's NewUserGuide listens for it.
      if (command.action === 'open-new-user-guide') openNewUserGuide();
      else if (command.path) onNavigate(command.path);
    }
  };

  const pickRecent = (value: string): void => {
    setQuery(value);
    setSelectedIndex(0);
    inputRef.current?.focus();
  };

  const clearKbScope = (): void => {
    setKbScope(null);
    setSelectedIndex(0);
    inputRef.current?.focus();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setSelectedIndex((current) => {
        const next = nextSelectedIndex(current, 1, flatCount);
        scrollToSelected(next);
        return next;
      });
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setSelectedIndex((current) => {
        const next = nextSelectedIndex(current, -1, flatCount);
        scrollToSelected(next);
        return next;
      });
    } else if (paletteShortcutDigit(event) !== undefined) {
      // ⌘1-9 — jump straight to the Nth visible item (Vue GlobalCommandPalette.vue:
      // 508-520). Digits only work while the palette is open; out-of-range
      // digits are a no-op WITHOUT preventDefault (Vue's `if (item)` guard).
      const index = paletteShortcutDigit(event)! - 1;
      if (index >= flatCount) return;
      event.preventDefault();
      runFlat(index);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      // The retrieval drawer is layered on top of the palette (Vue renders
      // the t-drawer inside the t-dialog): the first Escape dismisses the
      // drawer and leaves the palette open.
      if (retrievalDrawerVisible && retrievalSettings !== null) setRetrievalDrawerVisible(false);
      else onClose();
    } else if (event.key === 'Backspace' && !query && kbScope) {
      // Escape the scope chip from the keyboard, mirroring Vue's
      // onInputKeyDown Backspace-on-empty rule.
      event.preventDefault();
      clearKbScope();
    } else if (event.key === 'Enter') {
      event.preventDefault();
      runFlat(selectedIndex, true);
    }
  };

  // Empty state: with live search, only after the fan-out settled with zero
  // hits (Vue: !loading && hasSearched && !hasAnyResults). Without a client
  // keep the immediate commands-only empty copy.
  const hasAnyResults = flatCount > 0;
  const showEmpty = trimmed
    && (searchClient
      ? (!live.loading && live.hasSearched && !hasAnyResults)
      : items.length === 0);

  const itemRowClass = (selected: boolean): string =>
    // T15：旧栈 utility 串语义化为 .wk-cmdk-item-row（含 hover/选中态，platform-u.css）。
    `cmdk__item wk-cmdk-item-row${selected ? ' wk-cmdk-item-row--selected' : ''}`;
  const shortcutBadge = (digit: number | undefined, selected: boolean): ReactNode =>
    digit !== undefined ? (
      <span className={`cmdk__item-shortcut wk-cmdk-36 ${selected ? 'wk-cmdk-37' : 'wk-cmdk-38'}`}><kbd>⌘</kbd><kbd>{digit}</kbd></span>
    ) : null;

  const GroupShell = ({ label, count, children }: { label: string; count?: number; children?: ReactNode }): ReactNode => (
    <div className="wk-cmdk-2">
      <div className="wk-cmdk-3">
        <span>{label}</span>
        {typeof count === 'number' && <span className="wk-cmdk-4">{count}</span>}
      </div>
      {children}
    </div>
  );

  const chunkBase = recentCount;
  const messageBase = chunkBase + flatChunkItems.length;
  const kbBase = messageBase + flatMessageItems.length;
  const agentBase = kbBase + kbMatches.length;
  const sessionBase = agentBase + agentMatches.length;
  const commandBase = sessionBase + sessionMatches.length;

  return (
    <div
      className="wk-cmdk-5"
      role="presentation"
      onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}
    >
      <div className="cmdk wk-cmdk-6" role="dialog" aria-modal="true" aria-label={t('commandPalette.placeholder')} onKeyDown={onKeyDown}>
        <div className="wk-cmdk-7">
          {kbScope && (
            <span className="cmdk__scope-chip wk-cmdk-8" title={kbScope.name}>
              <span className="wk-cmdk-9">{kbScope.name}</span>
              <button
                type="button"
                data-cmdk-scope-remove
                className="wk-cmdk-10"
                aria-label={t('commandPalette.scope.remove')}
                title={t('commandPalette.scope.remove')}
                onClick={clearKbScope}
              >
                ×
              </button>
            </span>
          )}
          <input
            ref={inputRef}
            type="text"
            className="cmdk__input wk-cmdk-11"
            value={query}
            onChange={(event) => { setQuery(event.target.value); setSelectedIndex(0); }}
            placeholder={kbScope ? t('commandPalette.scope.placeholder') : t('commandPalette.placeholder')}
            spellCheck={false}
            autoFocus
          />
          {live.loading && (
            <span data-cmdk-loading aria-live="polite" className="wk-cmdk-12" />
          )}
          {retrievalSettings !== null && (
            <button type="button" data-cmdk-retrieval-trigger className={`wk-cmdk-39 ${retrievalDrawerVisible ? 'wk-cmdk-40' : 'wk-cmdk-41'}`} title={t('commandPalette.retrieval')} aria-label={t('commandPalette.retrieval')} onClick={() => setRetrievalDrawerVisible(true)}>
              ⚙
            </button>
          )}
          <button type="button" className="wk-cmdk-13" aria-label={t('commandPalette.hotkey.esc')} onClick={onClose}>
            ×
          </button>
        </div>
        <div className="wk-cmdk-14" ref={resultsRef}>
          {recentCount > 0 && (
            <div className="wk-cmdk-2">
              <div className="wk-cmdk-3">
                <span>{t('commandPalette.group.recent')}</span>
                <button type="button" className="wk-cmdk-15" onClick={onClearRecent}>
                  {t('commandPalette.clearRecent')}
                </button>
              </div>
              {recentQueries.map((value, index) => {
                const digit = shortcutDigitFor(index);
                return (
                  <button
                    key={`recent-${value}`}
                    type="button"
                    data-cmdk-index={index}
                    className={itemRowClass(selectedIndex === index)}
                    onMouseEnter={() => setSelectedIndex(index)}
                    onClick={() => pickRecent(value)}
                  >
                    <span className="wk-cmdk-16">{value}</span>
                    {shortcutBadge(digit, selectedIndex === index)}
                  </button>
                );
              })}
            </div>
          )}

          {/* ── Live search groups (Vue order: chunks → messages → kbs →
              agents → sessions); scoped mode renders chunks only. ── */}
          {trimmed && flatChunkItems.length > 0 && (
            <div className="wk-cmdk-2">
              <div className="wk-cmdk-3">
                <span>{t('commandPalette.group.chunks')}</span>
                {live.totalChunks > 0 && <span className="wk-cmdk-4">{live.totalChunks}</span>}
              </div>
              {flatChunkItems.map((item, index) => {
                const flatIndex = chunkBase + index;
                const digit = shortcutDigitFor(flatIndex);
                return (
                  <button
                    key={`chunk-${item.file.knowledgeId}-${item.chunk.id}`}
                    type="button"
                    data-cmdk-index={flatIndex}
                    className={`${itemRowClass(selectedIndex === flatIndex)} wk-cmdk-42`}
                    onMouseEnter={() => setSelectedIndex(flatIndex)}
                    onClick={() => runFlat(flatIndex)}
                  >
                    <span className="wk-cmdk-17">
                      <span className="wk-cmdk-18">{item.file.title}</span>
                      {item.file.kbName && <span className="cmdk-chunk-kb wk-cmdk-19">{item.file.kbName}</span>}
                      <span className={`wk-cmdk-43 ${item.chunk.matchType === 'vector' ? 'wk-cmdk-44' : 'wk-cmdk-45'}`}>
                        {item.chunk.matchType === 'vector' ? t('commandPalette.match.vector') : t('commandPalette.match.keyword')}
                      </span>
                      {item.chunk.score > 0 && <span className="wk-cmdk-20">{item.chunk.score.toFixed(2)}</span>}
                      {shortcutBadge(digit, selectedIndex === flatIndex)}
                    </span>
                    <span className="wk-cmdk-21">
                      <Highlighted text={item.chunk.matchedContent || item.chunk.content} query={trimmed} />
                    </span>
                  </button>
                );
              })}
            </div>
          )}

          {trimmed && flatMessageItems.length > 0 && (
            <div className="wk-cmdk-2">
              <div className="wk-cmdk-3">
                <span>{t('commandPalette.group.messages')}</span>
                {live.totalMessages > 0 && <span className="wk-cmdk-4">{live.totalMessages}</span>}
              </div>
              {flatMessageItems.map((item, index) => {
                const flatIndex = messageBase + index;
                return (
                  <button
                    key={`msg-${item.msg.requestId}`}
                    type="button"
                    data-cmdk-index={flatIndex}
                    className={`${itemRowClass(selectedIndex === flatIndex)} wk-cmdk-42`}
                    onMouseEnter={() => setSelectedIndex(flatIndex)}
                    onClick={() => runFlat(flatIndex)}
                  >
                    <span className="wk-cmdk-22">
                      {item.group.sessionTitle || t('commandPalette.untitledSession')}
                    </span>
                    <span className="wk-cmdk-21">
                      <span className="wk-cmdk-23">{item.msg.queryContent ? 'Q' : 'A'}</span>
                      <Highlighted text={item.msg.queryContent || item.msg.answerContent} query={trimmed} />
                    </span>
                  </button>
                );
              })}
            </div>
          )}

          {trimmed && kbMatches.length > 0 && (
            <GroupShell label={t('commandPalette.group.kbs')}>
              {kbMatches.map((kb, index) => {
                const flatIndex = kbBase + index;
                return (
                  <button
                    key={`kb-${kb.id}`}
                    type="button"
                    data-cmdk-index={flatIndex}
                    className={itemRowClass(selectedIndex === flatIndex)}
                    onMouseEnter={() => setSelectedIndex(flatIndex)}
                    onClick={() => runFlat(flatIndex)}
                  >
                    <span className="wk-cmdk-16">{kb.name}</span>
                    {shortcutBadge(shortcutDigitFor(flatIndex), selectedIndex === flatIndex)}
                  </button>
                );
              })}
            </GroupShell>
          )}

          {trimmed && agentMatches.length > 0 && (
            <GroupShell label={t('commandPalette.group.agents')}>
              {agentMatches.map((agent, index) => {
                const flatIndex = agentBase + index;
                return (
                  <button
                    key={`agent-${agent.id}`}
                    type="button"
                    data-cmdk-index={flatIndex}
                    className={itemRowClass(selectedIndex === flatIndex)}
                    onMouseEnter={() => setSelectedIndex(flatIndex)}
                    onClick={() => runFlat(flatIndex)}
                  >
                    <span className="wk-cmdk-16">{agent.name}</span>
                    {shortcutBadge(shortcutDigitFor(flatIndex), selectedIndex === flatIndex)}
                  </button>
                );
              })}
            </GroupShell>
          )}

          {trimmed && sessionMatches.length > 0 && (
            <GroupShell label={t('commandPalette.group.sessionsByTitle')}>
              {sessionMatches.map((session, index) => {
                const flatIndex = sessionBase + index;
                return (
                  <button
                    key={`session-${session.id}`}
                    type="button"
                    data-cmdk-index={flatIndex}
                    className={itemRowClass(selectedIndex === flatIndex)}
                    onMouseEnter={() => setSelectedIndex(flatIndex)}
                    onClick={() => runFlat(flatIndex)}
                  >
                    <span className="wk-cmdk-16">{session.title}</span>
                    {shortcutBadge(shortcutDigitFor(flatIndex), selectedIndex === flatIndex)}
                  </button>
                );
              })}
            </GroupShell>
          )}

          {showCommandsGroup && (
            <div className="wk-cmdk-2">
              <div className="wk-cmdk-3">
                <span>{trimmed ? t('commandPalette.group.commands') : t('commandPalette.group.quickActions')}</span>
              </div>
              {items.map((command, index) => {
                const flatIndex = commandBase + index;
                const digit = shortcutDigitFor(flatIndex);
                return (
                  <button
                    key={command.id}
                    type="button"
                    data-cmdk-index={flatIndex}
                    className={itemRowClass(selectedIndex === flatIndex)}
                    onMouseEnter={() => setSelectedIndex(flatIndex)}
                    onClick={() => runFlat(flatIndex)}
                  >
                    <span className="wk-cmdk-16">{t(command.labelKey)}</span>
                    {shortcutBadge(digit, selectedIndex === flatIndex)}
                  </button>
                );
              })}
            </div>
          )}
          {showEmpty && (
            <div className="cmdk__empty wk-cmdk-24">
              <p className="wk-cmdk-25">{t('commandPalette.empty.noResults')}</p>
              {/* Vue GlobalCommandPalette.vue:128-137 — empty-state actions:
                  ask AI with the current query / adjust retrieval settings. */}
              <div className="wk-cmdk-26">
                <button
                  type="button"
                  data-cmdk-ask-ai
                  className="wk-cmdk-27"
                  onClick={askAi}
                >
                  {t('commandPalette.empty.askAi')}
                </button>
                {retrievalSettings !== null && (
                  <button
                    type="button"
                    data-cmdk-adjust-retrieval
                    className="wk-cmdk-28"
                    onClick={() => setRetrievalDrawerVisible(true)}
                  >
                    {t('commandPalette.empty.adjustRetrieval')}
                  </button>
                )}
              </div>
            </div>
          )}
          {/* Vue GlobalCommandPalette.vue:142-150 — hotkey hint footer. */}
          <div className="wk-cmdk-29">
            <span className="wk-cmdk-30"><kbd>↑</kbd><kbd>↓</kbd> {t('commandPalette.hotkey.select')}</span>
            <span className="wk-cmdk-30"><kbd>↵</kbd> {t('commandPalette.hotkey.enter')}</span>
            <span className="wk-cmdk-30"><kbd>⌘</kbd><kbd>1</kbd>-<kbd>9</kbd> {t('commandPalette.hotkey.cmdNumber')}</span>
            <span className="wk-cmdk-30"><kbd>⌘</kbd><kbd>↵</kbd> {t('commandPalette.hotkey.cmdEnter')}</span>
            <span className="wk-cmdk-30"><kbd>Esc</kbd> {t('commandPalette.hotkey.esc')}</span>
          </div>
        </div>
      </div>
      {/* R465-A2 — retrieval-settings drawer (Vue lines 153-157): a 420px
          right panel layered on top of the palette, hosting the shell-provided
          RetrievalSettings surface. Overlay click / Esc / ✕ dismiss the drawer
          without closing the palette underneath. */}
      {retrievalDrawerVisible && retrievalSettings !== null && (
        <div
          data-testid="cmdk-retrieval-overlay"
          className="wk-cmdk-31"
          role="presentation"
          onMouseDown={(event) => { if (event.target === event.currentTarget) setRetrievalDrawerVisible(false); }}
          onKeyDown={(event) => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            setRetrievalDrawerVisible(false);
          }}
        >
          <aside
            data-testid="cmdk-retrieval-drawer"
            role="dialog"
            aria-modal="true"
            aria-label={t('retrievalSettings.title')}
            className="wk-cmdk-32"
          >
            <div className="wk-cmdk-33">
              <span className="wk-cmdk-34">{t('retrievalSettings.title')}</span>
              <button type="button" className="wk-cmdk-13" aria-label={t('commandPalette.hotkey.esc')} onClick={() => setRetrievalDrawerVisible(false)}>
                ×
              </button>
            </div>
            <div className="wk-cmdk-35">{retrievalSettings}</div>
          </aside>
        </div>
      )}
    </div>
  );
}
