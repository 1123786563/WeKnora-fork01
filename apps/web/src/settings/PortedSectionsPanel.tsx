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

const NOT_YET_PORTED_NOTE = '此分区的完整编辑器尚未迁移（not yet ported）。以下是已接入 API 的只读数据；编辑能力为后续工作，不会静默缺失。';

export function PortedSectionsPanel({ section }: { section: string }) {
  return <Card data-testid={'ported-panel-' + section}><p className="wk-muted">{NOT_YET_PORTED_NOTE}</p></Card>;
}

export function LiveSectionsPanel({ client, section, payload }: { client: WeKnoraClient; section: string; payload: unknown }) {
  void client;
  const rows = rowsOf(payload);
  const totals = payload !== null && typeof payload === 'object' && !Array.isArray(payload) ? (payload as Record<string, unknown>) : {};
  return <Card data-testid={'live-panel-' + section}>
    <p className="wk-muted">{NOT_YET_PORTED_NOTE}</p>
    {section === 'runtime-queues' && typeof totals.available === 'boolean'
      ? <Status tone={totals.available ? 'success' : 'error'}>{totals.available ? 'Runtime queues are available.' : 'Runtime queues are unavailable.'}</Status>
      : null}
    {rows.length === 0
      ? <p className="wk-settings-read-note">No rows were returned by the API for this section.</p>
      : <ul className="wk-list">{rows.map((row, index) => (
        <li key={index}>
          <strong>{summary(row)}</strong>
          <dl className="wk-settings-values">{settingsValueEntries(row).slice(0, 8).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl>
        </li>
      ))}</ul>}
  </Card>;
}
