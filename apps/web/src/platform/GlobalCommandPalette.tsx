import * as React from 'react';
import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { formatMessage, type Locale } from '@weknora/i18n';
import { Input } from '@weknora/ui';
import {
  COMMANDS,
  filterCommands,
  nextSelectedIndex,
  paletteShortcutDigit,
  shortcutDigitFor,
  type CommandDescriptor,
} from './command-palette.ts';
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
        part.hit ? <mark key={index} className="bg-transparent font-semibold text-[#1f2733]">{part.text}</mark> : <span key={index}>{part.text}</span>,
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
  } = props;
  const t = (key: string): string => formatMessage(locale, key);
  const [query, setQuery] = useState(initialQuery);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [kbScope, setKbScope] = useState<{ id: string; name: string } | null>(initialKbScope);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const resultsRef = useRef<HTMLDivElement | null>(null);

  const live = usePaletteLiveSearch({
    client: searchClient,
    query,
    enabled: open,
    scopeKbIds: useMemo(() => (kbScope ? [kbScope.id] : []), [kbScope]),
    debounceMs: searchDebounceMs,
  });

  useEffect(() => {
    if (!open) return;
    setQuery(initialQuery);
    setSelectedIndex(0);
    setKbScope(initialKbScope);
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
  const items: CommandDescriptor[] = useMemo(
    () => (trimmed ? filterCommands(COMMANDS, trimmed, t) : [...COMMANDS]),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [trimmed, locale],
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
      onNavigate(command.path);
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
      onClose();
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
    `cmdk__item flex w-full cursor-pointer items-center gap-2 border-none bg-transparent px-3.5 py-2 text-left text-sm text-[#1f2733] hover:bg-[#f2f5f9] ${selected ? 'bg-[#f2f5f9]' : ''}`;
  const shortcutBadge = (digit: number | undefined, selected: boolean): ReactNode =>
    digit !== undefined ? (
      <span className={`cmdk__item-shortcut inline-flex shrink-0 items-center gap-0.5 text-[10px] text-[rgba(0,0,0,0.4)] transition-opacity duration-100 [&_kbd]:inline-block [&_kbd]:min-w-[14px] [&_kbd]:rounded-[3px] [&_kbd]:border [&_kbd]:border-[#e7e7e7] [&_kbd]:bg-[#f3f3f3] [&_kbd]:px-1 [&_kbd]:leading-[14px] [&_kbd]:text-center [&_kbd]:text-[rgba(0,0,0,0.6)] ${selected ? 'opacity-100' : 'opacity-55'}`}><kbd>⌘</kbd><kbd>{digit}</kbd></span>
    ) : null;

  const GroupShell = ({ label, count, children }: { label: string; count?: number; children?: ReactNode }): ReactNode => (
    <div className="border-t border-[#f2f5f9] first:border-t-0">
      <div className="flex items-center justify-between px-3.5 py-1.5 text-xs text-[#8a94a3]">
        <span>{label}</span>
        {typeof count === 'number' && <span className="text-[11px]">{count}</span>}
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
      className="fixed inset-0 z-[1000] flex items-start justify-center bg-[rgba(15,23,32,0.45)] pt-[10vh]"
      role="presentation"
      onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}
    >
      <div className="cmdk flex max-h-[70vh] w-[640px] max-w-[calc(100vw-32px)] flex-col overflow-hidden rounded-[10px] bg-white shadow-[0_20px_60px_rgba(15,23,32,0.35)]" role="dialog" aria-modal="true" aria-label={t('commandPalette.placeholder')} onKeyDown={onKeyDown}>
        <div className="flex items-center gap-2 border-b border-[#eef1f5] px-3.5 py-3">
          {kbScope && (
            <span className="cmdk__scope-chip inline-flex h-[26px] max-w-[220px] shrink-0 items-center gap-1.5 rounded bg-[#f2f5f9] px-2 text-xs font-medium text-[#1f2733]" title={kbScope.name}>
              <span className="truncate">{kbScope.name}</span>
              <button
                type="button"
                data-cmdk-scope-remove
                className="flex h-5 w-5 cursor-pointer items-center justify-center border-none bg-transparent text-xs leading-none text-[#8a94a3] hover:text-[#1f2733]"
                aria-label={t('commandPalette.scope.remove')}
                title={t('commandPalette.scope.remove')}
                onClick={clearKbScope}
              >
                ×
              </button>
            </span>
          )}
          <Input
            ref={inputRef}
            type="text"
            className="cmdk__input min-w-0 flex-1 border-none bg-transparent text-[15px] text-[#1f2733] outline-none"
            value={query}
            onChange={(event) => { setQuery(event.target.value); setSelectedIndex(0); }}
            placeholder={kbScope ? t('commandPalette.scope.placeholder') : t('commandPalette.placeholder')}
            spellCheck={false}
            autoFocus
          />
          {live.loading && (
            <span data-cmdk-loading aria-live="polite" className="inline-block h-3.5 w-3.5 shrink-0 animate-spin rounded-full border-2 border-[#d7dde5] border-t-[#2f6fed]" />
          )}
          <button type="button" className="cursor-pointer rounded-md border-none bg-transparent px-1.5 py-1 text-lg leading-none text-[#8a94a3] hover:bg-[#f2f5f9] hover:text-[#1f2733]" aria-label={t('commandPalette.hotkey.esc')} onClick={onClose}>
            ×
          </button>
        </div>
        <div className="overflow-y-auto py-1.5" ref={resultsRef}>
          {recentCount > 0 && (
            <div className="border-t border-[#f2f5f9] first:border-t-0">
              <div className="flex items-center justify-between px-3.5 py-1.5 text-xs text-[#8a94a3]">
                <span>{t('commandPalette.group.recent')}</span>
                <button type="button" className="cursor-pointer border-none bg-transparent p-0 text-xs text-[#2f6fed]" onClick={onClearRecent}>
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
                    <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{value}</span>
                    {shortcutBadge(digit, selectedIndex === index)}
                  </button>
                );
              })}
            </div>
          )}

          {/* ── Live search groups (Vue order: chunks → messages → kbs →
              agents → sessions); scoped mode renders chunks only. ── */}
          {trimmed && flatChunkItems.length > 0 && (
            <div className="border-t border-[#f2f5f9] first:border-t-0">
              <div className="flex items-center justify-between px-3.5 py-1.5 text-xs text-[#8a94a3]">
                <span>{t('commandPalette.group.chunks')}</span>
                {live.totalChunks > 0 && <span className="text-[11px]">{live.totalChunks}</span>}
              </div>
              {flatChunkItems.map((item, index) => {
                const flatIndex = chunkBase + index;
                const digit = shortcutDigitFor(flatIndex);
                return (
                  <button
                    key={`chunk-${item.file.knowledgeId}-${item.chunk.id}`}
                    type="button"
                    data-cmdk-index={flatIndex}
                    className={`${itemRowClass(selectedIndex === flatIndex)} flex-col items-start gap-0.5`}
                    onMouseEnter={() => setSelectedIndex(flatIndex)}
                    onClick={() => runFlat(flatIndex)}
                  >
                    <span className="flex w-full min-w-0 items-center gap-2">
                      <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-sm font-medium">{item.file.title}</span>
                      {item.file.kbName && <span className="cmdk-chunk-kb shrink-0 rounded bg-[#f2f5f9] px-1.5 py-px text-[11px] font-normal text-[#8a94a3]">{item.file.kbName}</span>}
                      <span className={`shrink-0 rounded px-1.5 py-px text-[10px] font-medium ${item.chunk.matchType === 'vector' ? 'bg-[#eaf1fe] text-[#2f6fed]' : 'bg-[#f2f5f9] text-[#8a94a3]'}`}>
                        {item.chunk.matchType === 'vector' ? t('commandPalette.match.vector') : t('commandPalette.match.keyword')}
                      </span>
                      {item.chunk.score > 0 && <span className="shrink-0 text-[10px] text-[#b3bcc7]">{item.chunk.score.toFixed(2)}</span>}
                      {shortcutBadge(digit, selectedIndex === flatIndex)}
                    </span>
                    <span className="line-clamp-2 w-full text-left text-xs text-[#5f6b7a]">
                      <Highlighted text={item.chunk.matchedContent || item.chunk.content} query={trimmed} />
                    </span>
                  </button>
                );
              })}
            </div>
          )}

          {trimmed && flatMessageItems.length > 0 && (
            <div className="border-t border-[#f2f5f9] first:border-t-0">
              <div className="flex items-center justify-between px-3.5 py-1.5 text-xs text-[#8a94a3]">
                <span>{t('commandPalette.group.messages')}</span>
                {live.totalMessages > 0 && <span className="text-[11px]">{live.totalMessages}</span>}
              </div>
              {flatMessageItems.map((item, index) => {
                const flatIndex = messageBase + index;
                return (
                  <button
                    key={`msg-${item.msg.requestId}`}
                    type="button"
                    data-cmdk-index={flatIndex}
                    className={`${itemRowClass(selectedIndex === flatIndex)} flex-col items-start gap-0.5`}
                    onMouseEnter={() => setSelectedIndex(flatIndex)}
                    onClick={() => runFlat(flatIndex)}
                  >
                    <span className="min-w-0 w-full overflow-hidden text-ellipsis whitespace-nowrap text-sm font-medium">
                      {item.group.sessionTitle || t('commandPalette.untitledSession')}
                    </span>
                    <span className="line-clamp-2 w-full text-left text-xs text-[#5f6b7a]">
                      <span className="mr-1.5 inline-block rounded bg-[#f2f5f9] px-1 py-px text-[10px] font-semibold text-[#8a94a3]">{item.msg.queryContent ? 'Q' : 'A'}</span>
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
                    <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{kb.name}</span>
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
                    <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{agent.name}</span>
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
                    <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{session.title}</span>
                    {shortcutBadge(shortcutDigitFor(flatIndex), selectedIndex === flatIndex)}
                  </button>
                );
              })}
            </GroupShell>
          )}

          {showCommandsGroup && (
            <div className="border-t border-[#f2f5f9] first:border-t-0">
              <div className="flex items-center justify-between px-3.5 py-1.5 text-xs text-[#8a94a3]">
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
                    <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{t(command.labelKey)}</span>
                    {shortcutBadge(digit, selectedIndex === flatIndex)}
                  </button>
                );
              })}
            </div>
          )}
          {showEmpty && <p className="px-3.5 py-4 text-[13px] text-[#8a94a3]">{t('commandPalette.empty.noResults')}</p>}
          {/* Vue GlobalCommandPalette.vue:142-150 — hotkey hint footer. */}
          <div className="flex flex-wrap gap-4 border-t border-[#e7e7e7] px-3.5 py-2 text-[11px] text-[rgba(0,0,0,0.4)]">
            <span className="inline-flex items-center gap-1 [&_kbd]:inline-block [&_kbd]:min-w-4 [&_kbd]:rounded-[3px] [&_kbd]:border [&_kbd]:border-[#e7e7e7] [&_kbd]:bg-[#f3f3f3] [&_kbd]:px-[5px] [&_kbd]:py-px [&_kbd]:text-center [&_kbd]:text-[10px] [&_kbd]:leading-[14px] [&_kbd]:text-[rgba(0,0,0,0.6)]"><kbd>↑</kbd><kbd>↓</kbd> {t('commandPalette.hotkey.select')}</span>
            <span className="inline-flex items-center gap-1 [&_kbd]:inline-block [&_kbd]:min-w-4 [&_kbd]:rounded-[3px] [&_kbd]:border [&_kbd]:border-[#e7e7e7] [&_kbd]:bg-[#f3f3f3] [&_kbd]:px-[5px] [&_kbd]:py-px [&_kbd]:text-center [&_kbd]:text-[10px] [&_kbd]:leading-[14px] [&_kbd]:text-[rgba(0,0,0,0.6)]"><kbd>↵</kbd> {t('commandPalette.hotkey.enter')}</span>
            <span className="inline-flex items-center gap-1 [&_kbd]:inline-block [&_kbd]:min-w-4 [&_kbd]:rounded-[3px] [&_kbd]:border [&_kbd]:border-[#e7e7e7] [&_kbd]:bg-[#f3f3f3] [&_kbd]:px-[5px] [&_kbd]:py-px [&_kbd]:text-center [&_kbd]:text-[10px] [&_kbd]:leading-[14px] [&_kbd]:text-[rgba(0,0,0,0.6)]"><kbd>⌘</kbd><kbd>1</kbd>-<kbd>9</kbd> {t('commandPalette.hotkey.cmdNumber')}</span>
            <span className="inline-flex items-center gap-1 [&_kbd]:inline-block [&_kbd]:min-w-4 [&_kbd]:rounded-[3px] [&_kbd]:border [&_kbd]:border-[#e7e7e7] [&_kbd]:bg-[#f3f3f3] [&_kbd]:px-[5px] [&_kbd]:py-px [&_kbd]:text-center [&_kbd]:text-[10px] [&_kbd]:leading-[14px] [&_kbd]:text-[rgba(0,0,0,0.6)]"><kbd>⌘</kbd><kbd>↵</kbd> {t('commandPalette.hotkey.cmdEnter')}</span>
            <span className="inline-flex items-center gap-1 [&_kbd]:inline-block [&_kbd]:min-w-4 [&_kbd]:rounded-[3px] [&_kbd]:border [&_kbd]:border-[#e7e7e7] [&_kbd]:bg-[#f3f3f3] [&_kbd]:px-[5px] [&_kbd]:py-px [&_kbd]:text-center [&_kbd]:text-[10px] [&_kbd]:leading-[14px] [&_kbd]:text-[rgba(0,0,0,0.6)]"><kbd>Esc</kbd> {t('commandPalette.hotkey.esc')}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
