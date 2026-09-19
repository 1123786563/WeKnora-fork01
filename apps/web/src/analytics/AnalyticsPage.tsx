// SP11 Task 9 — admin analytics dashboard (/platform/analytics).
// Skeleton follows OrganizationsPage.tsx (props { client, role }, useState +
// load() + useEffect, formatMessage via the resolved locale, Tailwind class
// constants); data comes from the Task 6 client.analytics.* endpoints. All
// date math is delegated to analytics-range.ts so the UTC-day / 366-day
// contracts stay unit-tested. RBAC here is UI-only — the server route guard
// on /api/v1/analytics/* remains the real boundary (owner/admin pages in,
// contributor/viewer get the no-permission placeholder).
import { useEffect, useMemo, useState } from 'react';
import {
  Bar, BarChart, CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts';
import type { WeKnoraClient } from '@weknora/api-client';
import type { ActiveUsersPoint, AgentUsagePoint, ChannelSessionsPoint, QueryTrendPoint } from '@weknora/contracts';
import { formatMessage, isLocale } from '@weknora/i18n';
import { usePreferredLocale } from '../locale.ts';
import { clampAnalyticsRange, defaultAnalyticsRange, type AnalyticsDateRange } from './analytics-range.ts';

/** Tenant membership role, mirroring scopeRuntime.role() (router wiring). */
export type AnalyticsRole = 'owner' | 'admin' | 'contributor' | 'viewer';

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

// Query trend: queries rides the accent green, feedback splits blue/red.
const AN_TREND_COLORS: Record<string, string> = { queries: '#07c05f', likes: '#2e6de6', dislikes: '#d54941' };
// Channel stack palette cycles beyond the fourth source.
const AN_SERIES_COLORS = ['#07c05f', '#2e6de6', '#7c4dff', '#faad14', '#13c2c2', '#e37318', '#8b97a8'];
const AN_AGENT_COLORS: Record<string, string> = { messages: '#07c05f', unique_users: '#7c4dff' };

function t(locale: string, key: string): string {
  return formatMessage(isLocale(locale) ? locale : 'en-US', key);
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

  function applyRange() {
    const next = clampAnalyticsRange(fromInput, toInput);
    setFromInput(next.startTime);
    setToInput(next.endTime);
    setRange(next);
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
                {(['queries', 'likes', 'dislikes'] as const).map((key) => (
                  <Line key={key} type="monotone" dataKey={key} name={t(locale, `analytics.${key}`)} stroke={AN_TREND_COLORS[key]} strokeWidth={2} dot={false} />
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
                {(['messages', 'unique_users'] as const).map((key) => (
                  <Line key={key} type="monotone" dataKey={key} name={t(locale, `analytics.${key}`)} stroke={AN_AGENT_COLORS[key]} strokeWidth={2} dot={false} />
                ))}
              </LineChart>
            </ResponsiveContainer>
          ))}
        </section>
      </div>
    </main>
  );
}
