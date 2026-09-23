import type { WeKnoraClient } from '@weknora/api-client';
import { WkCard as Card, WkStatus as Status } from '../shared/wk-legacy.tsx';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import { settingsValueEntries } from './surface.ts';

const LOCALE_STORAGE_KEY = 'locale';

export function readInitialLocale(): Locale {
  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  return stored && isLocale(stored) ? stored : 'zh-CN';
}

export function useSettingsLocale(): Locale {
  return readInitialLocale();
}

export function settingsT(locale: Locale): (key: string, values?: Record<string, string | number>) => string {
  return (key, values) => formatMessage(locale, key, values);
}

function rowsOf(payload: unknown): Array<Record<string, unknown>> {
  const candidate = payload !== null && typeof payload === 'object' && !Array.isArray(payload) && Array.isArray((payload as Record<string, unknown>).items)
    ? (payload as Record<string, unknown>).items
    : payload;
  return Array.isArray(candidate)
    ? candidate.filter((item): item is Record<string, unknown> => item !== null && typeof item === 'object' && !Array.isArray(item))
    : [];
}

function summary(row: Record<string, unknown>): string {
  const preferred = ['name', 'key', 'queue', 'username', 'action', 'id'];
  for (const field of preferred) {
    const value = row[field];
    if (typeof value === 'string' && value) return value;
    if (typeof value === 'number') return String(value);
  }
  return '(unnamed)';
}

const NOT_YET_PORTED_NOTE = () => formatMessage(readInitialLocale(), 'settings.notYetPorted');

export function PortedSectionsPanel({ section }: { section: string }) {
  return <Card data-testid={'ported-panel-' + section}><p className="wk-ported-note">{NOT_YET_PORTED_NOTE()}</p></Card>;
}

export function LiveSectionsPanel({ client, section, payload }: { client: WeKnoraClient; section: string; payload: unknown }) {
  void client;
  const rows = rowsOf(payload);
  const totals = payload !== null && typeof payload === 'object' && !Array.isArray(payload) ? (payload as Record<string, unknown>) : {};
  return <Card data-testid={'live-panel-' + section}>
    <p className="wk-ported-note">{NOT_YET_PORTED_NOTE()}</p>
    {section === 'runtime-queues' && typeof totals.available === 'boolean'
      ? <Status tone={totals.available ? 'success' : 'error'}>{totals.available ? 'Runtime queues are available.' : 'Runtime queues are unavailable.'}</Status>
      : null}
    {rows.length === 0
      ? <p className="wk-settings-read-note">No rows were returned by the API for this section.</p>
      : <ul className="wk-list">{rows.map((row, index) => (
        <li key={index} className="wk-list-row">
          <strong>{summary(row)}</strong>
          <dl className="wk-settings-values">{settingsValueEntries(row).slice(0, 8).map(([key, value]) => <div className="wk-settings-values-row" key={key}><dt className="wk-settings-values-key">{key}</dt><dd>{value}</dd></div>)}</dl>
        </li>
      ))}</ul>}
  </Card>;
}
