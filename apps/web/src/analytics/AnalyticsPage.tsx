// SP11 Task 9 — admin analytics dashboard (/platform/analytics).
// SP12 Task 8 — the page body became a two-panel Tabs ('charts' keeps the
// SP11 four-chart grid untouched; 'usage' adds the admin byUser table with
// 0-based pagination and the CSV export download).
// Skeleton follows OrganizationsPage.tsx (props { client, role }, useState +
// load() + useEffect, formatMessage via the resolved locale, Tailwind class
// constants); data comes from the Task 6 client.analytics.* endpoints and
// SP12's client.usage.byUser/exportCsv. All date math is delegated to
// analytics-range.ts so the UTC-day / 366-day contracts stay unit-tested.
// RBAC here is UI-only — the server route guard on /api/v1/analytics/* and
// /api/v1/admin/usage/* remains the real boundary (owner/admin pages in,
// contributor/viewer get the no-permission placeholder).
import { useEffect, useMemo, useState } from 'react';
import {
  Bar, BarChart, CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts';
import type { WeKnoraClient, ClientBinaryResponse } from '@weknora/api-client';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@weknora/ui';
import type { ActiveUsersPoint, AgentUsagePoint, ChannelSessionsPoint, QueryTrendPoint, UsageByUserRow } from '@weknora/contracts';
import { formatMessage, isLocale } from '@weknora/i18n';
import { usePreferredLocale } from '../locale.ts';
import { AGENT_SERIES, TREND_SERIES } from './chart-series.ts';
import { clampAnalyticsRange, defaultAnalyticsRange, type AnalyticsDateRange } from './analytics-range.ts';

/** Tenant membership role, mirroring scopeRuntime.role() (router wiring). */
export type AnalyticsRole = 'owner' | 'admin' | 'contributor' | 'viewer';

/** SP12 Task 8 — fixed page size for the byUser table (server default, max 200). */
export const USAGE_PAGE_SIZE = 50;

/**
 * SP12 Task 8 — the by-user table shows the highest spend first. Rows arrive
 * server-ordered (window buckets per user × model); Array#sort is stable
 * (ES2019+), so equal-cost rows keep the server order. Pure: returns a new
 * array and never mutates the input (pinned by usage-tab.test.tsx).
 */
export function sortUsageRows(rows: readonly UsageByUserRow[]): UsageByUserRow[] {
  return [...rows].sort((a, b) => b.cost_microcredits - a.cost_microcredits);
}

// Both transports (xhr shim / createJsonTransport) lowercase response header
// names, so content-disposition is always the lowercase key. filename*=UTF-8''
// (RFC 5987) wins over a plain/bare filename= token.
function csvDownloadName(response: Pick<ClientBinaryResponse, 'headers'>, range: AnalyticsDateRange): string {
  const disposition = response.headers['content-disposition'] ?? '';
  const encoded = /filename\*=(?:utf-8'')?([^;]+)/i.exec(disposition);
  if (encoded) {
    try { return decodeURIComponent(encoded[1].trim().replace(/^"|"$/g, '')); } catch { /* fall through to plain */ }
  }
  const plain = /filename="([^"]+)"|filename=([^;]+)/i.exec(disposition);
  if (plain) return (plain[1] ?? plain[2] ?? '').trim().replace(/^"|"$/g, '');
  return `usage-${range.startTime}_${range.endTime}.csv`;
}

/* Tailwind v4 utility recipes shared across the page's cards and controls. */
const AN_PAGE = 'wk-page box-border h-full overflow-y-auto px-[28px] pt-[24px] pb-[32px]';
const AN_HEADER = 'mb-[20px] flex flex-col gap-[12px]';
const AN_TITLE = 'm-0 text-[24px] font-semibold leading-[32px] text-[rgba(23,26,29,0.92)]';
const AN_SUBTITLE = 'm-0 text-[14px] font-normal leading-[20px] text-[rgba(23,26,29,0.6)]';
const AN_FILTERS = 'flex flex-wrap items-center gap-[8px]';
const AN_DATE_INPUT = 'box-border h-[32px] rounded-[6px] border border-[#e7e7ea] bg-surface px-[8px] font-[inherit] text-[13px] text-[rgba(23,26,29,0.92)] focus:border-accent focus:outline-none';
const AN_BTN_PRIMARY = 'box-border inline-flex h-[32px] cursor-pointer items-center justify-center rounded-[3px] border-0 bg-accent px-[15px] font-[inherit] text-[14px] font-medium text-white shadow-[0_2px_8px_rgba(7,192,95,0.25)] [transition:all_.2s_ease] hover:shadow-[0_4px_14px_rgba(7,192,95,0.35)] disabled:cursor-not-allowed disabled:opacity-55';
const AN_BTN_OUTLINE = 'box-border inline-flex h-[32px] cursor-pointer items-center justify-center rounded-[3px] border border-[rgba(7,192,95,0.5)] bg-surface px-[15px] font-[inherit] text-[14px] font-medium text-accent [transition:all_.2s_ease] hover:border-accent hover:bg-accent-wash';
const AN_TEXT_INPUT = 'box-border h-[32px] w-[220px] rounded-[6px] border border-[#e7e7ea] bg-surface px-[10px] font-[inherit] text-[13px] text-[rgba(23,26,29,0.92)] placeholder:text-[rgba(23,26,29,0.35)] focus:border-accent focus:outline-none';
const AN_GRID = 'grid grid-cols-1 gap-[16px] min-[1100px]:grid-cols-2';
const AN_CARD = 'box-border rounded-[10px] border border-[#e7e7ea] bg-surface px-[16px] py-[14px] shadow-[0_1px_3px_rgba(0,0,0,0.04)]';
const AN_CARD_TITLE = 'm-0 mb-[8px] text-[15px] font-semibold text-[rgba(23,26,29,0.92)]';
const AN_STATE = 'flex h-[240px] items-center justify-center text-[13px] text-[rgba(23,26,29,0.4)]';
const AN_ERROR = 'flex h-[240px] flex-col items-center justify-center gap-[10px] text-[13px] text-[#d54941]';
const AN_AGENT_BAR = 'mb-[16px] flex flex-wrap items-center gap-[8px]';
const AN_FORBIDDEN = 'flex h-full min-h-[240px] flex-col items-center justify-center gap-[8px] px-[20px] py-[60px] text-center';
// SP12 Task 8 — by-user table, same recipes as the settings UsagePanel so
// both usage surfaces read as one feature.
const AN_USAGE_NOTE = 'm-0 mb-[12px] text-[13px] text-[rgba(23,26,29,0.6)]';
const AN_USAGE_BAR = 'mb-[12px] flex flex-wrap items-center gap-[8px]';
const AN_USAGE_EXPORT_ERROR = 'text-[13px] text-[#d54941]';
const AN_USAGE_TABLE = 'w-full border-collapse text-[13px]';
const AN_USAGE_CELL = 'border-b border-[#eef1f5] px-[10px] py-[8px] text-left';
const AN_USAGE_NUM = AN_USAGE_CELL + ' text-right tabular-nums';
const AN_USAGE_PAGER = 'mt-[12px] flex flex-wrap items-center gap-[8px] text-[13px] text-[rgba(23,26,29,0.6)]';
const AN_BTN_PAGER = 'box-border inline-flex h-[28px] cursor-pointer items-center justify-center rounded-[3px] border border-[rgba(7,192,95,0.5)] bg-surface px-[10px] font-[inherit] text-[12px] font-medium text-accent [transition:all_.2s_ease] hover:border-accent hover:bg-accent-wash disabled:cursor-not-allowed disabled:opacity-55';

// Channel stack palette cycles beyond the fourth source.
const AN_SERIES_COLORS = ['#07c05f', '#2e6de6', '#7c4dff', '#faad14', '#13c2c2', '#e37318', '#8b97a8'];

function t(locale: string, key: string, values?: Record<string, string | number>): string {
  return formatMessage(isLocale(locale) ? locale : 'en-US', key, values);
}

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

// Channels arrive as flat (date, source, sessions) points; the stacked chart
// wants one row per date with a column per source. An empty source is the
// backend's web bucket, so it keeps the 'web' legend entry.
function channelSourceOf(source: string): string {
  const trimmed = source.trim();
  return trimmed === '' ? 'web' : trimmed;
}

export function AnalyticsPage({ client, role }: { client: WeKnoraClient; role?: AnalyticsRole }) {
  const locale = usePreferredLocale();
  const [range, setRange] = useState<AnalyticsDateRange>(() => defaultAnalyticsRange());
  const [fromInput, setFromInput] = useState(() => defaultAnalyticsRange().startTime);
  const [toInput, setToInput] = useState(() => defaultAnalyticsRange().endTime);
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState('');
  const [trendItems, setTrendItems] = useState<QueryTrendPoint[]>([]);
  const [userItems, setUserItems] = useState<ActiveUsersPoint[]>([]);
  const [channelItems, setChannelItems] = useState<ChannelSessionsPoint[]>([]);
  // Agent section: on-demand — the id is only committed to a query on the
  // 查询 button, then refetched whenever the applied range changes.
  const [agentInput, setAgentInput] = useState('');
  const [agentQueryId, setAgentQueryId] = useState('');
  const [agentItems, setAgentItems] = useState<AgentUsagePoint[]>([]);
  const [agentLoading, setAgentLoading] = useState(false);
  const [agentError, setAgentError] = useState('');
  // SP12 Task 8 — usage tab: lazy dataset (fetched only while the tab is
  // active), a 0-based page cursor and the CSV export download state.
  const [tab, setTab] = useState<'charts' | 'usage'>('charts');
  const [usageItems, setUsageItems] = useState<UsageByUserRow[]>([]);
  const [usageLoading, setUsageLoading] = useState(false);
  const [usageError, setUsageError] = useState('');
  const [usagePage, setUsagePage] = useState(0);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState('');

  // Same gate as the nav entry (canViewChannelSessions: owner/admin only);
  // viewers and contributors get the placeholder instead of API 403 noise.
  const canView = role === 'owner' || role === 'admin';

  const rangeParams = useMemo(() => ({ startTime: range.startTime, endTime: range.endTime }), [range.startTime, range.endTime]);

  async function loadAgentUsage(agentId: string, params: { startTime: string; endTime: string }) {
    setAgentLoading(true);
    setAgentError('');
    try {
      const result = await client.analytics.agentUsage(agentId, params);
      setAgentItems(result.items);
    } catch (reason) {
      setAgentItems([]);
      setAgentError(errorText(reason, t(locale, 'common.error')));
    } finally {
      setAgentLoading(false);
    }
  }

  async function load() {
    setLoading(true);
    setListError('');
    try {
      // The three range-scoped datasets load concurrently; the agent series
      // stays on-demand (empty until an id is committed below).
      const [trend, users, channels] = await Promise.all([
        client.analytics.queryTrend(rangeParams),
        client.analytics.activeUsers(rangeParams),
        client.analytics.channelSessions(rangeParams),
      ]);
      setTrendItems(trend.items);
      setUserItems(users.items);
      setChannelItems(channels.items);
    } catch (reason) {
      setListError(errorText(reason, t(locale, 'common.error')));
    } finally {
      setLoading(false);
    }
    if (agentQueryId) void loadAgentUsage(agentQueryId, rangeParams);
  }

  // Date-range changes reload everything: the applied range (not the draft
  // inputs) is the effect input, so typing in a picker never fires requests.
  useEffect(() => {
    if (!canView) return;
    void load();
    /* eslint-disable-line react-hooks/exhaustive-deps */
  }, [client, rangeParams, canView]);

  async function loadUsage(page: number) {
    setUsageLoading(true);
    setUsageError('');
    try {
      const result = await client.usage.byUser({ page, pageSize: USAGE_PAGE_SIZE, ...rangeParams });
      setUsageItems(result.items);
    } catch (reason) {
      setUsageItems([]);
      setUsageError(errorText(reason, t(locale, 'common.error')));
    } finally {
      setUsageLoading(false);
    }
  }

  // The usage dataset is on-demand: fetched when the tab activates and again
  // whenever the shared applied range or the page cursor moves (page is
  // 0-based — page 0 is the first page, per client.usage.byUser).
  useEffect(() => {
    if (!canView || tab !== 'usage') return;
    void loadUsage(usagePage);
    /* eslint-disable-line react-hooks/exhaustive-deps */
  }, [client, tab, usagePage, rangeParams, canView]);

  // SP12 Task 8 — CSV export: rides requestBinary (chat artifact download
  // precedent) and turns the body + content-disposition into a browser
  // download; failures surface inline next to the button.
  async function exportUsageCsv() {
    setExporting(true);
    setExportError('');
    try {
      const response = await client.usage.exportCsv(rangeParams);
      const blob = typeof Blob !== 'undefined' && response.body instanceof Blob
        ? response.body
        : new Blob([response.body], { type: response.contentType || 'text/csv' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = csvDownloadName(response, range);
      anchor.click();
      URL.revokeObjectURL(url);
    } catch (reason) {
      setExportError(errorText(reason, t(locale, 'common.error')));
    } finally {
      setExporting(false);
    }
  }

  function applyRange() {
    const next = clampAnalyticsRange(fromInput, toInput);
    setFromInput(next.startTime);
    setToInput(next.endTime);
    setRange(next);
    // A fresh window always restarts the usage table from the first page.
    setUsagePage(0);
  }

  function submitAgentQuery() {
    const id = agentInput.trim();
    if (!id) return;
    setAgentQueryId(id);
    void loadAgentUsage(id, rangeParams);
  }

  // Pivot the flat channel points into date rows × source columns.
  const channelRows = useMemo<Array<Record<string, number | string>>>(() => {
    const countsByDate = new Map<string, Record<string, number>>();
    for (const point of channelItems) {
      const source = channelSourceOf(point.source);
      const counts = countsByDate.get(point.date) ?? {};
      counts[source] = (counts[source] ?? 0) + point.sessions;
      countsByDate.set(point.date, counts);
    }
    return [...countsByDate.entries()]
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      .map(([date, counts]) => ({ date, ...counts }));
  }, [channelItems]);
  const channelSources = useMemo(() => {
    const sources = new Set<string>();
    for (const point of channelItems) sources.add(channelSourceOf(point.source));
    return [...sources].sort();
  }, [channelItems]);
  // Flat by-user rows, highest cost first (sortUsageRows is pure + stable).
  const usageRows = useMemo(() => sortUsageRows(usageItems), [usageItems]);

  if (!canView) {
    return (
      <main className={AN_PAGE}>
        <div className={AN_FORBIDDEN} role="alert">
          <h2 className={AN_TITLE}>{t(locale, 'menu.analytics')}</h2>
          <p className={AN_SUBTITLE}>{t(locale, 'analytics.noPermission')}</p>
        </div>
      </main>
    );
  }

  // Shared three-state body for every chart card (OrganizationsPage's
  // loading/listError/empty flow, one card at a time).
  const chartBody = (empty: boolean, error: string, isLoading: boolean, onRetry: () => void, chart: React.ReactNode): React.ReactNode => {
    if (isLoading) return <div className={AN_STATE} role="status">{t(locale, 'common.loading')}</div>;
    if (error) {
      return (
        <div className={AN_ERROR} role="alert">
          <span>{error}</span>
          <button type="button" className={AN_BTN_OUTLINE} onClick={onRetry}>{t(locale, 'common.retry')}</button>
        </div>
      );
    }
    if (empty) return <div className={AN_STATE}>{t(locale, 'common.empty')}</div>;
    return chart;
  };

  const dateTick = (value: string): string => (typeof value === 'string' && value.length >= 10 ? value.slice(5, 10) : String(value));
  const axisTick = { fontSize: 11, fill: 'rgba(23,26,29,0.55)' };

  return (
    <main className={AN_PAGE}>
      <header className={AN_HEADER}>
        <div>
          <h2 className={AN_TITLE}>{t(locale, 'menu.analytics')}</h2>
          <p className={AN_SUBTITLE}>{t(locale, 'analytics.subtitle')}</p>
        </div>
        <div className={AN_FILTERS}>
          <label className="flex items-center gap-[6px] text-[13px] text-[rgba(23,26,29,0.6)]">
            {t(locale, 'analytics.rangeFrom')}
            <input type="date" className={AN_DATE_INPUT} value={fromInput} onChange={(event) => setFromInput(event.target.value)} />
          </label>
          <label className="flex items-center gap-[6px] text-[13px] text-[rgba(23,26,29,0.6)]">
            {t(locale, 'analytics.rangeTo')}
            <input type="date" className={AN_DATE_INPUT} value={toInput} onChange={(event) => setToInput(event.target.value)} />
          </label>
          <button type="button" className={AN_BTN_PRIMARY} onClick={applyRange}>{t(locale, 'analytics.apply')}</button>
        </div>
      </header>

      {/* SP12 Task 8 — 图表/用量 two panels; the date range above is shared,
          so changing it (or switching tabs) refetches the active dataset. */}
      <Tabs value={tab} onValueChange={(value) => setTab(value === 'usage' ? 'usage' : 'charts')}>
        <TabsList aria-label={t(locale, 'menu.analytics')}>
          <TabsTrigger value="charts">{t(locale, 'analytics.tabCharts')}</TabsTrigger>
          <TabsTrigger value="usage">{t(locale, 'analytics.usageTab')}</TabsTrigger>
        </TabsList>

        <TabsContent value="charts">
          <div className={AN_GRID}>
            <section className={AN_CARD}>
              <h3 className={AN_CARD_TITLE}>{t(locale, 'analytics.queryTrend')}</h3>
              {chartBody(trendItems.length === 0, listError, loading, () => void load(), (
                <ResponsiveContainer width="100%" height={240}>
                  <LineChart data={trendItems} margin={{ top: 8, right: 12, bottom: 0, left: -16 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#e7e7ea" />
                    <XAxis dataKey="date" tick={axisTick} tickFormatter={dateTick} minTickGap={24} />
                    <YAxis tick={axisTick} allowDecimals={false} />
                    <Tooltip />
                    <Legend wrapperStyle={{ fontSize: 12 }} />
                    {TREND_SERIES.map((series) => (
                      <Line key={series.dataKey} type="monotone" dataKey={series.dataKey} name={t(locale, series.labelKey)} stroke={series.color} strokeWidth={2} dot={false} />
                    ))}
                  </LineChart>
                </ResponsiveContainer>
              ))}
            </section>

            <section className={AN_CARD}>
              <h3 className={AN_CARD_TITLE}>{t(locale, 'analytics.activeUsers')}</h3>
              {chartBody(userItems.length === 0, listError, loading, () => void load(), (
                <ResponsiveContainer width="100%" height={240}>
                  <BarChart data={userItems} margin={{ top: 8, right: 12, bottom: 0, left: -16 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#e7e7ea" />
                    <XAxis dataKey="date" tick={axisTick} tickFormatter={dateTick} minTickGap={24} />
                    <YAxis tick={axisTick} allowDecimals={false} />
                    <Tooltip />
                    <Bar dataKey="active_users" name={t(locale, 'analytics.activeUsers')} fill="#2e6de6" radius={[2, 2, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              ))}
            </section>

            <section className={AN_CARD}>
              <h3 className={AN_CARD_TITLE}>{t(locale, 'analytics.channelSessions')}</h3>
              {chartBody(channelRows.length === 0, listError, loading, () => void load(), (
                <ResponsiveContainer width="100%" height={240}>
                  <BarChart data={channelRows} margin={{ top: 8, right: 12, bottom: 0, left: -16 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#e7e7ea" />
                    <XAxis dataKey="date" tick={axisTick} tickFormatter={dateTick} minTickGap={24} />
                    <YAxis tick={axisTick} allowDecimals={false} />
                    <Tooltip />
                    <Legend wrapperStyle={{ fontSize: 12 }} />
                    {channelSources.map((source, index) => (
                      <Bar key={source} dataKey={source} stackId="channels" fill={AN_SERIES_COLORS[index % AN_SERIES_COLORS.length]} />
                    ))}
                  </BarChart>
                </ResponsiveContainer>
              ))}
            </section>

            <section className={AN_CARD}>
              <h3 className={AN_CARD_TITLE}>{t(locale, 'analytics.agentUsage')}</h3>
              <div className={AN_AGENT_BAR}>
                <input
                  className={AN_TEXT_INPUT}
                  value={agentInput}
                  placeholder={t(locale, 'analytics.agentIdPlaceholder')}
                  onChange={(event) => setAgentInput(event.target.value)}
                  onKeyDown={(event) => { if (event.key === 'Enter') submitAgentQuery(); }}
                />
                <button type="button" className={AN_BTN_PRIMARY} disabled={agentInput.trim() === ''} onClick={submitAgentQuery}>{t(locale, 'analytics.agentQuery')}</button>
              </div>
              {chartBody(agentItems.length === 0, agentError, agentLoading, () => agentQueryId && void loadAgentUsage(agentQueryId, rangeParams), (
                <ResponsiveContainer width="100%" height={240}>
                  <LineChart data={agentItems} margin={{ top: 8, right: 12, bottom: 0, left: -16 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#e7e7ea" />
                    <XAxis dataKey="date" tick={axisTick} tickFormatter={dateTick} minTickGap={24} />
                    <YAxis tick={axisTick} allowDecimals={false} />
                    <Tooltip />
                    <Legend wrapperStyle={{ fontSize: 12 }} />
                    {AGENT_SERIES.map((series) => (
                      <Line key={series.dataKey} type="monotone" dataKey={series.dataKey} name={t(locale, series.labelKey)} stroke={series.color} strokeWidth={2} dot={false} />
                    ))}
                  </LineChart>
                </ResponsiveContainer>
              ))}
            </section>
          </div>
        </TabsContent>

        {/* SP12 Task 8 — 用量：admin 全员 byUser 平铺行（不聚合），cost 降序。 */}
        <TabsContent value="usage">
          <p className={AN_USAGE_NOTE}>{t(locale, 'analytics.usageDescription')}</p>
          <div className={AN_USAGE_BAR}>
            <button type="button" className={AN_BTN_OUTLINE} disabled={exporting} onClick={() => void exportUsageCsv()}>{t(locale, 'analytics.exportCsv')}</button>
            {exportError ? <span role="alert" className={AN_USAGE_EXPORT_ERROR}>{exportError}</span> : null}
          </div>
          <section className={AN_CARD}>
            {usageLoading ? (
              <div className={AN_STATE} role="status">{t(locale, 'common.loading')}</div>
            ) : usageError ? (
              <div className={AN_ERROR} role="alert">
                <span>{usageError}</span>
                <button type="button" className={AN_BTN_OUTLINE} onClick={() => void loadUsage(usagePage)}>{t(locale, 'common.retry')}</button>
              </div>
            ) : usageRows.length === 0 ? (
              <div className={AN_STATE}>{t(locale, 'common.empty')}</div>
            ) : (
              <table className={AN_USAGE_TABLE} data-testid="usage-by-user-table">
                <thead>
                  <tr className="border-b border-[#e7e7ea]">
                    <th className={AN_USAGE_CELL + ' font-semibold'}>{t(locale, 'analytics.colUser')}</th>
                    <th className={AN_USAGE_CELL + ' font-semibold'}>{t(locale, 'analytics.colModel')}</th>
                    <th className={AN_USAGE_CELL + ' font-semibold'}>{t(locale, 'analytics.colWindow')}</th>
                    <th className={AN_USAGE_NUM + ' font-semibold'}>{t(locale, 'analytics.colInput')}</th>
                    <th className={AN_USAGE_NUM + ' font-semibold'}>{t(locale, 'analytics.colOutput')}</th>
                    <th className={AN_USAGE_NUM + ' font-semibold'}>{t(locale, 'analytics.colCache')}</th>
                    <th className={AN_USAGE_NUM + ' font-semibold'}>{t(locale, 'analytics.colCost')}</th>
                  </tr>
                </thead>
                <tbody>
                  {usageRows.map((row, index) => (
                    // One row per user × model × window bucket; the index
                    // disambiguates when the backend buckets finer than a day.
                    <tr key={`${row.user_id}|${row.model}|${row.window_start}|${index}`}>
                      <td className={AN_USAGE_CELL}>{row.user_id}</td>
                      <td className={AN_USAGE_CELL}>{row.model}</td>
                      <td className={AN_USAGE_CELL}>{row.window_start}</td>
                      <td className={AN_USAGE_NUM}>{row.input_tokens.toLocaleString(locale)}</td>
                      <td className={AN_USAGE_NUM}>{row.output_tokens.toLocaleString(locale)}</td>
                      <td className={AN_USAGE_NUM}>{(row.cache_read_tokens + row.cache_write_tokens).toLocaleString(locale)}</td>
                      <td className={AN_USAGE_NUM}>{row.cost_microcredits.toLocaleString(locale)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            <div className={AN_USAGE_PAGER}>
              <button type="button" className={AN_BTN_PAGER} disabled={usageLoading || usagePage === 0} onClick={() => setUsagePage(usagePage - 1)}>{t(locale, 'analytics.prevPage')}</button>
              <span>{t(locale, 'analytics.pageOf', { page: usagePage + 1 })}</span>
              {/* A short page (< pageSize rows) is by definition the last one. */}
              <button type="button" className={AN_BTN_PAGER} disabled={usageLoading || usageItems.length < USAGE_PAGE_SIZE} onClick={() => setUsagePage(usagePage + 1)}>{t(locale, 'analytics.nextPage')}</button>
            </div>
          </section>
        </TabsContent>
      </Tabs>
    </main>
  );
}
