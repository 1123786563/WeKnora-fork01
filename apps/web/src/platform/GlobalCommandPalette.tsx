import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { formatMessage, type Locale } from '@weknora/i18n';
import {
  COMMANDS,
  filterCommands,
  nextSelectedIndex,
  paletteShortcutDigit,
  shortcutDigitFor,
  type CommandDescriptor,
} from './command-palette.ts';
import './command-palette.css';

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
      className="cmdk-overlay"
      role="presentation"
      onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}
    >
      <div className="cmdk" role="dialog" aria-modal="true" aria-label={t('commandPalette.placeholder')} onKeyDown={onKeyDown}>
        <div className="cmdk__input-row">
          <input
            ref={inputRef}
            type="text"
            className="cmdk__input"
            value={query}
            onChange={(event) => { setQuery(event.target.value); setSelectedIndex(0); }}
            placeholder={t('commandPalette.placeholder')}
            spellCheck={false}
            autoFocus
          />
          <button type="button" className="cmdk__icon-btn" aria-label={t('commandPalette.hotkey.esc')} onClick={onClose}>
            ×
          </button>
        </div>
        <div className="cmdk__results">
          {recentCount > 0 && (
            <div className="cmdk__group">
              <div className="cmdk__group-header">
                <span>{t('commandPalette.group.recent')}</span>
                <button type="button" className="cmdk__group-action" onClick={onClearRecent}>
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
                    className={`cmdk__item${selectedIndex === index ? ' cmdk__item--selected' : ''}`}
                    onMouseEnter={() => setSelectedIndex(index)}
                    onClick={() => pickRecent(value)}
                  >
                    <span className="cmdk__item-label">{value}</span>
                    {digit !== undefined && (
                      <span className="cmdk__item-shortcut"><kbd>⌘</kbd><kbd>{digit}</kbd></span>
                    )}
                  </button>
                );
              })}
            </div>
          )}
          <div className="cmdk__group">
            <div className="cmdk__group-header"><span>{groupLabel}</span></div>
            {items.map((command, index) => {
              const flatIndex = recentCount + index;
              const digit = shortcutDigitFor(flatIndex);
              return (
                <button
                  key={command.id}
                  type="button"
                  data-cmdk-index={flatIndex}
                  className={`cmdk__item${selectedIndex === flatIndex ? ' cmdk__item--selected' : ''}`}
                  onMouseEnter={() => setSelectedIndex(flatIndex)}
                  onClick={() => runCommand(command)}
                >
                  <span className="cmdk__item-label">{t(command.labelKey)}</span>
                  {digit !== undefined && (
                    <span className="cmdk__item-shortcut"><kbd>⌘</kbd><kbd>{digit}</kbd></span>
                  )}
                </button>
              );
            })}
            {trimmed && items.length === 0 && <p className="cmdk__empty">{t('commandPalette.empty.noResults')}</p>}
          </div>
        </div>
      </div>
    </div>
  );
}
