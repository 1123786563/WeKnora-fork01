import type { WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
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
  return <Card data-testid={'ported-panel-' + section}><p className="wk-muted text-muted">{NOT_YET_PORTED_NOTE()}</p></Card>;
}

export function LiveSectionsPanel({ client, section, payload }: { client: WeKnoraClient; section: string; payload: unknown }) {
  void client;
  const rows = rowsOf(payload);
  const totals = payload !== null && typeof payload === 'object' && !Array.isArray(payload) ? (payload as Record<string, unknown>) : {};
  return <Card data-testid={'live-panel-' + section}>
    <p className="wk-muted text-muted">{NOT_YET_PORTED_NOTE()}</p>
    {section === 'runtime-queues' && typeof totals.available === 'boolean'
      ? <Status tone={totals.available ? 'success' : 'error'}>{totals.available ? 'Runtime queues are available.' : 'Runtime queues are unavailable.'}</Status>
      : null}
    {rows.length === 0
      ? <p className="wk-settings-read-note text-muted-strong text-[.9rem]">No rows were returned by the API for this section.</p>
      : <ul className="wk-list m-0 list-none p-0">{rows.map((row, index) => (
        <li key={index} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]">
          <strong>{summary(row)}</strong>
          <dl className="wk-settings-values mb-0 mt-4 grid gap-[.65rem]">{settingsValueEntries(row).slice(0, 8).map(([key, value]) => <div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] gap-[.8rem] border-b border-line-soft py-[.55rem] max-[720px]:grid-cols-1 max-[720px]:gap-1" key={key}><dt className="text-muted-strong font-[650] [overflow-wrap:anywhere]">{key}</dt><dd className="m-0 font-mono text-[.85rem] [overflow-wrap:anywhere] whitespace-pre-wrap">{value}</dd></div>)}</dl>
        </li>
      ))}</ul>}
  </Card>;
}
