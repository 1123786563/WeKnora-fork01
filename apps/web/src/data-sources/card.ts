import type { DataSource } from '@weknora/api-client';

// Port of frontend/src/utils/cronHumanize.ts: known cron schedule presets map
// to human-readable i18n keys whose values are byte-exact against the Vue
// `datasource.scheduleHuman.*` baseline. Unknown patterns fall back to the raw
// cron expression; an empty schedule renders as '--'.
const CRON_PRESET_KEYS: Record<string, string> = {
  '0 */30 * * * *': 'dataSource.scheduleHuman.30min',
  '0 0 * * * *': 'dataSource.scheduleHuman.1h',
  '0 0 */6 * * *': 'dataSource.scheduleHuman.6h',
  '0 0 */12 * * *': 'dataSource.scheduleHuman.12h',
  '0 0 2 * * *': 'dataSource.scheduleHuman.24h',
};

export function humanizeCron(cron: string | null | undefined, t: (key: string) => string): string {
  const key = cron ? CRON_PRESET_KEYS[cron] : undefined;
  if (key) return t(key);
  return cron || '--';
}

// Port of frontend/src/utils/cronHumanize.ts relativeTime: buckets for just
// now / minutes / hours / days, then a locale date past 30 days. `now` is
// injectable so tests stay deterministic.
export function relativeTime(
  ts: string | null | undefined,
  t: (key: string, values?: Record<string, string | number>) => string,
  now: number = Date.now(),
): string {
  if (!ts) return t('dataSource.neverSynced');
  const then = new Date(ts).getTime();
  if (isNaN(then)) return t('dataSource.neverSynced');

  const diffMs = now - then;
  if (diffMs < 60000) return t('dataSource.justNow');

  const minutes = Math.floor(diffMs / 60000);
  if (minutes < 60) return t('dataSource.minutesAgo', { n: minutes });

  const hours = Math.floor(diffMs / 3600000);
  if (hours < 24) return t('dataSource.hoursAgo', { n: hours });

  const days = Math.floor(diffMs / 86400000);
  if (days < 30) return t('dataSource.daysAgo', { n: days });

  return new Date(ts).toLocaleDateString();
}

// Port of Vue DataSourceSettings.vue syncResultPills: only non-zero metrics
// become pills; created/updated/deleted carry +/~/- prefixes while failed and
// skipped counts carry their labels.
export type SyncResultPill = { text: string; kind: 'created' | 'updated' | 'deleted' | 'failed' | 'skipped' };

export function syncResultPills(
  source: Pick<DataSource, 'latest_sync_log'>,
  t: (key: string) => string,
): SyncResultPill[] {
  const log = source.latest_sync_log;
  if (!log) return [];
  const pills: SyncResultPill[] = [];
  if ((log.items_created ?? 0) > 0) pills.push({ text: `+${log.items_created}`, kind: 'created' });
  if ((log.items_updated ?? 0) > 0) pills.push({ text: `~${log.items_updated}`, kind: 'updated' });
  if ((log.items_deleted ?? 0) > 0) pills.push({ text: `-${log.items_deleted}`, kind: 'deleted' });
  if ((log.items_failed ?? 0) > 0) pills.push({ text: `${log.items_failed} ${t('dataSource.logMetric.failed')}`, kind: 'failed' });
  if ((log.items_skipped ?? 0) > 0) pills.push({ text: `${log.items_skipped} ${t('dataSource.logMetric.skipped')}`, kind: 'skipped' });
  return pills;
}
