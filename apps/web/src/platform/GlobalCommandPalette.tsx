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
}

/**
 * React port of frontend/src/components/GlobalCommandPalette.vue (R011/N003).
 *
 * Scope for this slice (S03): visible ⌘K dialog with recent-query history and
 * the static quick-action command catalogue, full keyboard navigation, and
 * the same open/close triggers as Vue (global ⌘K, `?cmdk=` deep link). Live
 * chunk/message/KB/agent search is intentionally out of scope — see
 * command-palette.ts for the rationale — so only command results are shown
 * once the user types a query.
 */
export function GlobalCommandPalette(props: GlobalCommandPaletteProps): ReactNode {
  const { open, initialQuery, recentQueries, locale, onClose, onNavigate, onSearch, onClearRecent } = props;
  const t = (key: string): string => formatMessage(locale, key);
  const [query, setQuery] = useState(initialQuery);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!open) return;
    setQuery(initialQuery);
    setSelectedIndex(0);
    const raf = requestAnimationFrame(() => inputRef.current?.focus());
    return () => cancelAnimationFrame(raf);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const trimmed = query.trim();
  const items: CommandDescriptor[] = useMemo(
    () => (trimmed ? filterCommands(COMMANDS, trimmed, t) : [...COMMANDS]),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [trimmed, locale],
  );
  const recentCount = trimmed ? 0 : recentQueries.length;
  const flatCount = recentCount + items.length;

  if (!open) return null;

  const runCommand = (cmd: CommandDescriptor): void => {
    if (trimmed) onSearch(trimmed);
    onClose();
    onNavigate(cmd.path);
  };

  const pickRecent = (value: string): void => {
    setQuery(value);
    setSelectedIndex(0);
    inputRef.current?.focus();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setSelectedIndex((current) => nextSelectedIndex(current, 1, flatCount));
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setSelectedIndex((current) => nextSelectedIndex(current, -1, flatCount));
    } else if (paletteShortcutDigit(event) !== undefined) {
      // ⌘1-9 — jump straight to the Nth visible item (Vue GlobalCommandPalette.vue:
      // 508-520). This handler lives on the dialog div, so digits only work while
      // the palette is open — deliberately no window binding (Round N+3 ruling:
      // the rejected app-wide ⌘1 must stay dead). Out-of-range digits are a
      // no-op WITHOUT preventDefault, mirroring Vue's `if (item)` guard, and
      // digits take precedence over ⌘Enter (a digit key can never be Enter).
      const digit = paletteShortcutDigit(event);
      if (digit === undefined) return;
      const index = digit - 1;
      if (index < recentCount) {
        const value = recentQueries[index];
        if (value === undefined) return;
        event.preventDefault();
        pickRecent(value);
        return;
      }
      const command = items[index - recentCount];
      if (!command) return;
      event.preventDefault();
      runCommand(command);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      onClose();
    } else if (event.key === 'Enter') {
      event.preventDefault();
      if (selectedIndex < recentCount) {
        const value = recentQueries[selectedIndex];
        if (value !== undefined) pickRecent(value);
        return;
      }
      const command = items[selectedIndex - recentCount];
      if (command) runCommand(command);
    }
  };

  const groupLabel = trimmed ? t('commandPalette.group.commands') : t('commandPalette.group.quickActions');

  return (
    <div
      className="fixed inset-0 z-[1000] flex items-start justify-center bg-[rgba(15,23,32,0.45)] pt-[10vh]"
      role="presentation"
      onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}
    >
      <div className="cmdk flex max-h-[70vh] w-[640px] max-w-[calc(100vw-32px)] flex-col overflow-hidden rounded-[10px] bg-white shadow-[0_20px_60px_rgba(15,23,32,0.35)]" role="dialog" aria-modal="true" aria-label={t('commandPalette.placeholder')} onKeyDown={onKeyDown}>
        <div className="flex items-center gap-2 border-b border-[#eef1f5] px-3.5 py-3">
          <Input
            ref={inputRef}
            type="text"
            className="cmdk__input min-w-0 flex-1 border-none bg-transparent text-[15px] text-[#1f2733] outline-none"
            value={query}
            onChange={(event) => { setQuery(event.target.value); setSelectedIndex(0); }}
            placeholder={t('commandPalette.placeholder')}
            spellCheck={false}
            autoFocus
          />
          <button type="button" className="cursor-pointer rounded-md border-none bg-transparent px-1.5 py-1 text-lg leading-none text-[#8a94a3] hover:bg-[#f2f5f9] hover:text-[#1f2733]" aria-label={t('commandPalette.hotkey.esc')} onClick={onClose}>
            ×
          </button>
        </div>
        <div className="overflow-y-auto py-1.5">
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
                    className={`cmdk__item flex w-full cursor-pointer items-center gap-2 border-none bg-transparent px-3.5 py-2 text-left text-sm text-[#1f2733] hover:bg-[#f2f5f9] ${selectedIndex === index ? 'bg-[#f2f5f9]' : ''}`}
                    onMouseEnter={() => setSelectedIndex(index)}
                    onClick={() => pickRecent(value)}
                  >
                    <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{value}</span>
                    {digit !== undefined && (
                      <span className={`cmdk__item-shortcut inline-flex shrink-0 items-center gap-0.5 text-[10px] text-[rgba(0,0,0,0.4)] transition-opacity duration-100 [&_kbd]:inline-block [&_kbd]:min-w-[14px] [&_kbd]:rounded-[3px] [&_kbd]:border [&_kbd]:border-[#e7e7e7] [&_kbd]:bg-[#f3f3f3] [&_kbd]:px-1 [&_kbd]:leading-[14px] [&_kbd]:text-center [&_kbd]:text-[rgba(0,0,0,0.6)] ${selectedIndex === index ? 'opacity-100' : 'opacity-55'}`}><kbd>⌘</kbd><kbd>{digit}</kbd></span>
                    )}
                  </button>
                );
              })}
            </div>
          )}
          <div className="border-t border-[#f2f5f9] first:border-t-0">
            <div className="flex items-center justify-between px-3.5 py-1.5 text-xs text-[#8a94a3]"><span>{groupLabel}</span></div>
            {items.map((command, index) => {
              const flatIndex = recentCount + index;
              const digit = shortcutDigitFor(flatIndex);
              return (
                <button
                  key={command.id}
                  type="button"
                  data-cmdk-index={flatIndex}
                  className={`cmdk__item flex w-full cursor-pointer items-center gap-2 border-none bg-transparent px-3.5 py-2 text-left text-sm text-[#1f2733] hover:bg-[#f2f5f9] ${selectedIndex === flatIndex ? 'bg-[#f2f5f9]' : ''}`}
                  onMouseEnter={() => setSelectedIndex(flatIndex)}
                  onClick={() => runCommand(command)}
                >
                  <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{t(command.labelKey)}</span>
                  {digit !== undefined && (
                    <span className={`cmdk__item-shortcut inline-flex shrink-0 items-center gap-0.5 text-[10px] text-[rgba(0,0,0,0.4)] transition-opacity duration-100 [&_kbd]:inline-block [&_kbd]:min-w-[14px] [&_kbd]:rounded-[3px] [&_kbd]:border [&_kbd]:border-[#e7e7e7] [&_kbd]:bg-[#f3f3f3] [&_kbd]:px-1 [&_kbd]:leading-[14px] [&_kbd]:text-center [&_kbd]:text-[rgba(0,0,0,0.6)] ${selectedIndex === flatIndex ? 'opacity-100' : 'opacity-55'}`}><kbd>⌘</kbd><kbd>{digit}</kbd></span>
                  )}
                </button>
              );
            })}
            {trimmed && items.length === 0 && <p className="px-3.5 py-4 text-[13px] text-[#8a94a3]">{t('commandPalette.empty.noResults')}</p>}
          </div>
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
