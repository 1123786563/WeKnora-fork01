// SP13 Task 7 — settings 查询审计分区面板（section=query-history，租户级
// Admin+）。Ruling P-1：隐私三档 radio（normal/anonymized/disabled，KV
// query-history-config）并入面板顶部；Admin 可写，非 admin 只读展示。
// 数据走 client.queryHistory（Task 6）：adminList(source=all 审计过滤) /
// snapshot(行点击抽屉) / startExport→exportStatus 2s 轮询→downloadExport。
// disabled 模式：挂载先读 KV，mode=disabled 时整面板只渲染占位，不再发审计
// 请求；审计请求 403（策略在面板挂载后被切为 disabled）同样兜底占位。
// 日期窗口复用 analytics-range.ts 的 defaultAnalyticsRange/clampAnalyticsRange。
import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { ApiError, type WeKnoraClient } from '@weknora/api-client';
import type {
  ChatMessage,
  MessageFeedbackRow,
  QueryHistoryFeedbackRating,
  QueryHistoryMode,
  QueryHistorySessionRow,
  QueryHistorySnapshot,
} from '@weknora/contracts';
import { formatMessage, type Locale } from '@weknora/i18n';
import { roleAtLeast, type SettingsRole } from '@weknora/views/settings/registry';
import { Button as TButton } from 'tdesign-react';
import { WkCard as Card, WkStatus as Status } from '../shared/wk-legacy.tsx';
import { clampAnalyticsRange, defaultAnalyticsRange, type AnalyticsDateRange } from '../analytics/analytics-range.ts';

export const QUERY_HISTORY_PAGE_SIZE = 50;
/** Export jobs are polled every 2s while pending/running (Task 4 async job). */
export const QUERY_HISTORY_EXPORT_POLL_MS = 2000;

export type QueryHistoryFeedbackFilter = 'all' | QueryHistoryFeedbackRating;

/** Applied (post-应用 button) audit filter; draft inputs live in panel state. */
export interface QueryHistoryFilter {
  readonly userId: string;
  readonly range: AnalyticsDateRange;
  readonly feedback: QueryHistoryFeedbackFilter;
}

// ---------------------------------------------------------------------------
// Pure helpers (exported for query-history-panel.test.tsx).

/** snapshot feedback rows → like/dislike counters for the drawer header. */
export function feedbackSummary(rows: readonly MessageFeedbackRow[]): { like: number; dislike: number } {
  let like = 0;
  let dislike = 0;
  for (const row of rows) {
    if (row.rating === 'like') like += 1;
    else if (row.rating === 'dislike') dislike += 1;
  }
  return { like, dislike };
}

/** Backend end_time is exclusive（session/handler.go parseSessionFilterTime,
 * 上界不含）; the picked end date must be covered, so shift it +1 day. */
export function shiftEndTimeExclusive(endTime: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(endTime)) return endTime;
  const shifted = new Date(`${endTime}T00:00:00Z`);
  shifted.setUTCDate(shifted.getUTCDate() + 1);
  return shifted.toISOString().slice(0, 10);
}

/** Applied filter + 1-based page → client.queryHistory.adminList params.
 * Backend Pagination.GetPage clamps anything below 1 up to 1 and PageResult
 * echoes the clamped value, so the panel keeps page state 1-based end to end
 * (initial state, indicator, prev/next bounds, and setPage(result.page)). */
export function queryHistoryListParams(filter: QueryHistoryFilter, page: number, pageSize: number = QUERY_HISTORY_PAGE_SIZE) {
  const userId = filter.userId.trim();
  return {
    source: 'all',
    page,
    pageSize,
    ...(userId ? { userId } : {}),
    startTime: filter.range.startTime,
    endTime: shiftEndTimeExclusive(filter.range.endTime),
    ...(filter.feedback === 'all' ? {} : { feedback: filter.feedback }),
  };
}

// ---------------------------------------------------------------------------
// 1-based pagination helpers (exported for query-history-panel.test.tsx).
// The backend's page contract is 1-based (Pagination.GetPage clamps <1 to 1
// and PageResult echoes the clamped page), so every helper and the panel's
// page state below are 1-based too.

/** Total pages for a row count; always at least 1 (an empty listing still
 * shows "第 1 / 1 页" rather than a zero-page indicator). */
export function queryHistoryTotalPages(total: number, pageSize: number = QUERY_HISTORY_PAGE_SIZE): number {
  return Math.max(1, Math.ceil(total / pageSize));
}

/** 1-based previous page, floored at the first page. */
export function prevPage(page: number): number {
  return Math.max(1, page - 1);
}

/** 1-based next page, capped at the last page. */
export function nextPage(page: number, totalPages: number): number {
  return Math.min(totalPages, page + 1);
}

/** Whether 上一页 is available (anything before the first page). */
export function canGoPrevPage(page: number): boolean {
  return page > 1;
}

/** Whether 下一页 is available; the LAST page must be reachable through the
 * next button — the boundary the old 0-based state got wrong once
 * setPage(result.page) started feeding the 1-based echo back in. */
export function canGoNextPage(page: number, totalPages: number): boolean {
  return page < totalPages;
}

/** Knowledge-citation count badge of a snapshot message. */
export function messageReferenceCount(message: ChatMessage): number {
  return Array.isArray(message.knowledge_references) ? message.knowledge_references.length : 0;
}

/** 403 = the tenant disabled query history (CheckQueryHistoryAccess
 * ForbiddenError) → the panel falls back to the disabled placeholder. */
export function isQueryHistoryDisabledError(reason: unknown): boolean {
  return reason instanceof ApiError && reason.status === 403;
}

/** created_at → {date, time}（SystemAuditLogPanel.auditDateParts 同语义）. */
export function queryHistoryDateParts(value: string | undefined, locale: string): { date: string; time: string } {
  if (!value) return { date: '-', time: '' };
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return { date: value, time: '' };
  return {
    date: new Intl.DateTimeFormat(locale, { year: 'numeric', month: '2-digit', day: '2-digit' }).format(date),
    time: new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(date),
  };
}

/** Session rows ride the open-shaped ChatSession contract; the backend's web
 * bucket is the empty source（AnalyticsPage.channelSourceOf 同映射）. */
export function sessionSourceText(row: QueryHistorySessionRow): string {
  const source = typeof row.source === 'string' ? row.source.trim() : '';
  return source === '' ? 'web' : source;
}

function errorText(reason: unknown, fallback: string): string { return reason instanceof Error ? reason.message : fallback; }

const QH_DATE_INPUT = 'box-border h-[32px] rounded-[6px] border border-[#e7e7ea] bg-surface px-[8px] font-[inherit] text-[13px] text-[rgba(23,26,29,0.92)] focus:border-accent focus:outline-none';
const QH_TEXT_INPUT = QH_DATE_INPUT + ' w-[180px]';
const QH_TABLE = 'w-full min-w-[680px] border-collapse text-[13px]';
const QH_TABLE_CELL = 'border-b border-[#eef1f5] px-[10px] py-[8px] text-left';
const PRIVACY_MODES: readonly QueryHistoryMode[] = ['normal', 'anonymized', 'disabled'];
const PRIVACY_TAG_TONES: Record<QueryHistoryMode, string> = {
  normal: 'bg-[rgba(7,192,95,0.1)] border-[rgba(7,192,95,0.24)] text-[#078a45]',
  anonymized: 'bg-[rgba(234,167,49,0.1)] border-[rgba(234,167,49,0.22)] text-[#9a6a0b]',
  disabled: 'bg-[rgba(220,60,60,0.08)] border-[rgba(220,60,60,0.2)] text-[#c03939]',
};

/** Snapshot drawer body (createPortal into document.body — SystemAuditLogPanel
 * 右侧抽屉模式) rendered by the panel; plain-text message content 首版. */
function QueryHistorySnapshotDrawer({ snapshot, locale, t, onClose }: { snapshot: QueryHistorySnapshot; locale: Locale; t: (key: string, values?: Record<string, string | number>) => string; onClose: () => void }) {
  const session = snapshot.session;
  const parts = queryHistoryDateParts(session.created_at, locale);
  const summary = feedbackSummary(snapshot.feedback);
  const meta: Array<[string, string]> = [
    [t('settings.queryHistory.colUser'), typeof session.user_id === 'string' && session.user_id ? session.user_id : '—'],
    [t('settings.queryHistory.colSource'), sessionSourceText(session)],
    [t('settings.queryHistory.colEngine'), typeof session.engine_type === 'string' && session.engine_type ? session.engine_type : '—'],
    [t('settings.queryHistory.colCreated'), parts.date === '-' ? '—' : `${parts.date} ${parts.time}`],
  ];
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [onClose]);
  return createPortal(
    <div className="fixed bottom-0 right-0 top-0 z-[3200] w-[min(640px,100vw)] max-w-[640px] overflow-y-auto border-l border-[rgba(120,135,155,0.25)] bg-white p-6 shadow-[-8px_0_24px_rgba(23,32,51,0.14)] [&_h4]:m-0 [&_h4]:mb-3 [&_h4]:mt-[18px] [&_h4]:border-t [&_h4]:border-[rgba(120,135,155,0.16)] [&_h4]:pt-[18px] [&_h4]:text-xs [&_h4]:font-semibold [&_h4]:tracking-[0.04em] [&_h4]:text-[#8a96a8] first:[&_h4]:mt-0" role="dialog" aria-modal="true" aria-label={t('settings.queryHistory.detailTitle')}>
      <div className="mb-4 flex items-start justify-between [&_h3]:m-0 [&_h3]:mb-1.5 [&_h3]:text-lg [&_h3]:font-semibold [&_p]:m-0 [&_p]:text-xs [&_p]:text-[#8a96a8] [&_button]:cursor-pointer [&_button]:border-0 [&_button]:bg-transparent [&_button]:text-2xl">
        <div className="min-w-0">
          <h3 className="break-all">{session.title}</h3>
          <p className="break-all font-mono">{session.id}</p>
        </div>
        <button type="button" aria-label={t('settings.queryHistory.close')} onClick={onClose}>×</button>
      </div>
      <h4>{t('settings.queryHistory.snapshotMeta')}</h4>
      <dl className="m-0 grid gap-2.5">
        {meta.map(([label, value]) => (
          <div key={label} className="border-b border-[rgba(120,135,155,0.18)] pb-2">
            <dt className="text-xs text-[#5c6b83]">{label}</dt>
            <dd className="m-0 mt-1 break-all">{value}</dd>
          </div>
        ))}
      </dl>
      <h4>{t('settings.queryHistory.snapshotMessages')}</h4>
      {snapshot.messages.length === 0 ? <p className="m-0 text-[13px] text-[#8a96a8]">{t('common.empty')}</p> : (
        <ol className="m-0 flex list-none flex-col gap-2.5 p-0">
          {snapshot.messages.map((message) => {
            const refs = messageReferenceCount(message);
            return (
              <li key={message.id} className="rounded-lg border border-[rgba(120,135,155,0.18)] bg-[#f7f9fb] p-3">
                <div className="mb-1.5 flex items-center gap-2">
                  <span className={`inline-flex shrink-0 whitespace-nowrap rounded-[999px] border px-[7px] text-xs leading-[18px] ${message.role === 'user' ? PRIVACY_TAG_TONES.normal : message.role === 'assistant' ? 'bg-[rgba(46,109,230,0.08)] border-[rgba(46,109,230,0.22)] text-[#2f5ca8]' : 'bg-[rgba(120,135,155,0.1)] border-[rgba(120,135,155,0.2)] text-[#5c6b83]'}`}>
                    {message.role === 'user' ? t('settings.queryHistory.roleUser') : message.role === 'assistant' ? t('settings.queryHistory.roleAssistant') : t('settings.queryHistory.roleSystem')}
                  </span>
                  {refs > 0 ? <span className="inline-flex shrink-0 items-center gap-1 rounded-[999px] border border-[rgba(120,135,155,0.2)] bg-[rgba(120,135,155,0.08)] px-[7px] text-xs leading-[18px] text-[#5c6b83]" data-testid="query-history-ref-badge">{t('settings.queryHistory.refCount', { n: refs })}</span> : null}
                </div>
                <p className="m-0 whitespace-pre-wrap break-all text-[13px] leading-[1.55]">{message.content}</p>
              </li>
            );
          })}
        </ol>
      )}
      {snapshot.truncated ? <p className="m-0 mt-2 text-xs text-[#9a6a0b]" role="note">{t('settings.queryHistory.truncated')}</p> : null}
      <h4>{t('settings.queryHistory.snapshotFeedback', { like: summary.like, dislike: summary.dislike })}</h4>
      {snapshot.feedback.length === 0 ? <p className="m-0 text-[13px] text-[#8a96a8]">{t('settings.queryHistory.noFeedback')}</p> : (
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {snapshot.feedback.map((row) => (
            <li key={row.id} className="rounded-lg border border-[rgba(120,135,155,0.18)] p-3">
              <div className="flex items-center gap-2 text-xs text-[#5c6b83]">
                <span className={`inline-flex shrink-0 whitespace-nowrap rounded-[999px] border px-[7px] leading-[18px] ${row.rating === 'like' ? PRIVACY_TAG_TONES.normal : PRIVACY_TAG_TONES.disabled}`}>{row.rating === 'like' ? t('settings.queryHistory.feedbackLike') : t('settings.queryHistory.feedbackDislike')}</span>
                <span className="break-all font-mono">{row.message_id}</span>
                <span className="break-all">{row.user_id}</span>
              </div>
              {typeof row.comment === 'string' && row.comment ? <p className="m-0 mt-1.5 whitespace-pre-wrap break-all text-[13px]">{row.comment}</p> : null}
            </li>
          ))}
        </ul>
      )}
    </div>,
    document.body,
  );
}

export function QueryHistoryPanel({ client, locale = 'zh-CN', role = 'owner' }: { client: WeKnoraClient; locale?: Locale; role?: SettingsRole }) {
  const t = (key: string, values?: Record<string, string | number>): string => formatMessage(locale, key, values);
  const canEditPolicy = roleAtLeast(role, 'admin');
  const initialRange = useMemo(() => defaultAnalyticsRange(), []);
  // Draft filter inputs — committed to `filter` only by the 应用 button.
  const [userIdInput, setUserIdInput] = useState('');
  const [fromInput, setFromInput] = useState(initialRange.startTime);
  const [toInput, setToInput] = useState(initialRange.endTime);
  const [feedbackInput, setFeedbackInput] = useState<QueryHistoryFeedbackFilter>('all');
  const [filter, setFilter] = useState<QueryHistoryFilter>({ userId: '', range: initialRange, feedback: 'all' });

  // Privacy policy (KV query-history-config): loaded first on mount;
  // mode=disabled renders the placeholder and never issues audit requests.
  const [mode, setMode] = useState<QueryHistoryMode | null>(null);
  const [policyLoading, setPolicyLoading] = useState(true);
  const [policySaving, setPolicySaving] = useState(false);
  const [policyError, setPolicyError] = useState<string | null>(null);
  const [disabledFallback, setDisabledFallback] = useState(false);

  const [rows, setRows] = useState<QueryHistorySessionRow[]>([]);
  const [total, setTotal] = useState(0);
  // 1-based, matching the backend's clamped PageResult.page echo (see the
  // pagination helpers above); setPage(result.page) stays consistent.
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const loadGeneration = useRef(0);

  const [snapshotSession, setSnapshotSession] = useState<QueryHistorySessionRow | null>(null);
  const [snapshot, setSnapshot] = useState<QueryHistorySnapshot | null>(null);
  const [snapshotLoading, setSnapshotLoading] = useState(false);
  const [snapshotError, setSnapshotError] = useState<string | null>(null);

  // Export job: start → 2s poll → done becomes a download button; the poll
  // interval is cleared on unmount and on every terminal state.
  const [exportJobId, setExportJobId] = useState<number | null>(null);
  const [exportPhase, setExportPhase] = useState<'idle' | 'starting' | 'polling' | 'done' | 'failed'>('idle');
  const [exportError, setExportError] = useState('');
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  function stopPolling() {
    if (pollRef.current !== null) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }

  useEffect(() => stopPolling, []);

  async function load(nextPage: number, nextFilter: QueryHistoryFilter) {
    const generation = ++loadGeneration.current;
    setLoading(true);
    setLoadError(null);
    try {
      const result = await client.queryHistory.adminList(queryHistoryListParams(nextFilter, nextPage));
      if (generation !== loadGeneration.current) return;
      setRows(result.data);
      setTotal(result.total);
      setPage(result.page);
    } catch (reason) {
      if (generation !== loadGeneration.current) return;
      setRows([]);
      setTotal(0);
      if (isQueryHistoryDisabledError(reason)) {
        // Policy flipped to disabled after mount (or the KV read failed on an
        // older backend): same placeholder as the mount-time KV check.
        setDisabledFallback(true);
      } else {
        setLoadError(errorText(reason, t('common.error')));
      }
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }

  // Mount: read the KV privacy policy first — disabled short-circuits the
  // audit listing entirely. A failed KV read (older backend without the
  // query-history-config case) proceeds; the listing's own 403 still lands in
  // the disabled placeholder.
  useEffect(() => {
    let cancelled = false;
    client.queryHistory.queryHistoryConfig.get()
      .then((config) => { if (!cancelled) setMode(config.mode); })
      .catch(() => { if (!cancelled) setMode(null); })
      .finally(() => { if (!cancelled) setPolicyLoading(false); });
    return () => { cancelled = true; };
    /* eslint-disable-line react-hooks/exhaustive-deps */
  }, [client]);

  // Audit listing: reloads on the committed filter/page, but only while the
  // effective mode is not disabled.
  useEffect(() => {
    if (policyLoading || mode === 'disabled') return;
    void load(page, filter);
    /* eslint-disable-line react-hooks/exhaustive-deps */
  }, [client, policyLoading, mode, filter, page]);

  async function openSnapshot(session: QueryHistorySessionRow) {
    setSnapshotSession(session);
    setSnapshot(null);
    setSnapshotError(null);
    setSnapshotLoading(true);
    try {
      const result = await client.queryHistory.snapshot(session.id);
      setSnapshot(result);
    } catch (reason) {
      setSnapshotError(errorText(reason, t('common.error')));
    } finally {
      setSnapshotLoading(false);
    }
  }

  async function updateMode(next: QueryHistoryMode) {
    if (!canEditPolicy || policySaving) return;
    setPolicySaving(true);
    setPolicyError(null);
    try {
      const config = await client.queryHistory.queryHistoryConfig.update(next);
      setMode(config.mode);
      if (config.mode !== 'disabled' && mode === 'disabled') {
        // Leaving the disabled state: the audit listing was never issued.
        setDisabledFallback(false);
        setPage(1);
      }
    } catch (reason) {
      setPolicyError(errorText(reason, t('common.error')));
    } finally {
      setPolicySaving(false);
    }
  }

  function applyFilter() {
    const next = clampAnalyticsRange(fromInput, toInput);
    setFromInput(next.startTime);
    setToInput(next.endTime);
    setPage(1);
    setFilter({ userId: userIdInput, range: next, feedback: feedbackInput });
  }

  async function pollExportStatus(jobId: number) {
    setExportPhase('polling');
    stopPolling();
    pollRef.current = setInterval(() => {
      void (async () => {
        try {
          const status = await client.queryHistory.exportStatus(jobId);
          if (status.status === 'done') { stopPolling(); setExportPhase('done'); }
          else if (status.status === 'failed') { stopPolling(); setExportPhase('failed'); setExportError(status.error_message || t('settings.queryHistory.exportFailed')); }
        } catch (reason) {
          stopPolling();
          setExportPhase('failed');
          setExportError(errorText(reason, t('settings.queryHistory.exportFailed')));
        }
      })();
    }, QUERY_HISTORY_EXPORT_POLL_MS);
  }

  async function startExport() {
    if (exportPhase === 'starting' || exportPhase === 'polling') return;
    setExportPhase('starting');
    setExportError('');
    try {
      const started = await client.queryHistory.startExport({
        ...(filter.userId.trim() ? { userId: filter.userId.trim() } : {}),
        startTime: filter.range.startTime,
        endTime: shiftEndTimeExclusive(filter.range.endTime),
        ...(filter.feedback === 'all' ? {} : { feedback: filter.feedback }),
      });
      setExportJobId(started.job_id);
      await pollExportStatus(started.job_id);
    } catch (reason) {
      setExportPhase('failed');
      setExportError(errorText(reason, t('settings.queryHistory.exportFailed')));
    }
  }

  // CSV download: rides requestBinary and turns the body into a browser
  // download (SP12 usage tab / AnalyticsPage.exportUsageCsv precedent).
  async function downloadExport() {
    if (exportJobId === null) return;
    try {
      const response = await client.queryHistory.downloadExport(exportJobId);
      const blob = typeof Blob !== 'undefined' && response.body instanceof Blob
        ? response.body
        : new Blob([response.body], { type: response.contentType || 'text/csv' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      const disposition = response.headers['content-disposition'] ?? '';
      const named = /filename\*=(?:utf-8'')?([^;]+)/i.exec(disposition) ?? /filename="([^"]+)"|filename=([^;]+)/i.exec(disposition);
      anchor.download = named ? (named[1] ?? named[2] ?? '').trim().replace(/^"|"$/g, '') : `query-history-${filter.range.startTime}_${filter.range.endTime}.csv`;
      anchor.click();
      URL.revokeObjectURL(url);
    } catch (reason) {
      setExportError(errorText(reason, t('settings.queryHistory.exportFailed')));
    }
  }

  function resetExport() {
    stopPolling();
    setExportJobId(null);
    setExportPhase('idle');
    setExportError('');
  }

  const totalPages = queryHistoryTotalPages(total);
  const effectiveMode: QueryHistoryMode | null = mode === 'disabled' || disabledFallback ? 'disabled' : mode;

  // Privacy radios rendered by both branches (audit view + disabled
  // placeholder): a plain JSX-returning closure keeps one definition without
  // remounting the radio group on every parent render.
  const privacyPolicyCard = (
    <Card data-testid="query-history-privacy" className="flex flex-col gap-2">
      <h3 className="m-0 text-[15px] font-semibold text-[rgba(23,26,29,0.92)]">{t('settings.queryHistory.privacyTitle')}</h3>
      <p className="m-0 text-[13px] text-[rgba(23,26,29,0.6)]">{t('settings.queryHistory.privacyDescription')}</p>
      <div className="flex flex-col gap-1.5" role="radiogroup" aria-label={t('settings.queryHistory.privacyTitle')}>
        {PRIVACY_MODES.map((candidate) => (
          <label key={candidate} className="flex items-center gap-2 text-[13px]">
            <input
              type="radio"
              name="query-history-mode"
              checked={(mode ?? 'normal') === candidate}
              disabled={!canEditPolicy || policySaving || mode === null}
              onChange={() => { void updateMode(candidate); }}
              data-testid={`query-history-mode-${candidate}`}
            />
            <span className={`inline-flex shrink-0 whitespace-nowrap rounded-[999px] border px-[7px] text-xs leading-[18px] ${PRIVACY_TAG_TONES[candidate]}`}>{t(`settings.queryHistory.mode_${candidate}`)}</span>
            <span className="text-[rgba(23,26,29,0.6)]">{t(`settings.queryHistory.mode_${candidate}_hint`)}</span>
          </label>
        ))}
      </div>
      {!canEditPolicy ? <p className="m-0 text-xs text-[#8a96a8]">{t('settings.queryHistory.privacyReadonly')}</p> : null}
      {policySaving ? <p className="m-0 text-xs text-[#8a96a8]">{t('settings.queryHistory.privacySaving')}</p> : null}
      {policyError ? <Status tone="error">{policyError}</Status> : null}
    </Card>
  );

  if (policyLoading) return <div data-testid="query-history-panel"><Status>{t('common.loading')}</Status></div>;

  if (effectiveMode === 'disabled') {
    return (
      <div data-testid="query-history-panel" className="flex flex-col gap-4">
        <Card data-testid="query-history-disabled">
          <Status>{t('settings.queryHistory.disabledPlaceholder')}</Status>
        </Card>
        {privacyPolicyCard}
      </div>
    );
  }

  return (
    <div data-testid="query-history-panel" className="flex flex-col gap-4">
      {privacyPolicyCard}

      <div className="flex flex-wrap items-center gap-[8px]">
        <label className="flex items-center gap-[6px] text-[13px] text-[rgba(23,26,29,0.6)]">
          {t('settings.queryHistory.filterUser')}
          <input type="text" className={QH_TEXT_INPUT} value={userIdInput} onChange={(value) => setUserIdInput(String(value))} data-testid="query-history-user-input" />
        </label>
        <label className="flex items-center gap-[6px] text-[13px] text-[rgba(23,26,29,0.6)]">
          {t('settings.usage.rangeFrom')}
          <input type="date" className={QH_DATE_INPUT} value={fromInput} onChange={(value) => setFromInput(String(value))} />
        </label>
        <label className="flex items-center gap-[6px] text-[13px] text-[rgba(23,26,29,0.6)]">
          {t('settings.usage.rangeTo')}
          <input type="date" className={QH_DATE_INPUT} value={toInput} onChange={(value) => setToInput(String(value))} />
        </label>
        <label className="flex items-center gap-[6px] text-[13px] text-[rgba(23,26,29,0.6)]">
          {t('settings.queryHistory.filterFeedback')}
          <select className={QH_DATE_INPUT} value={feedbackInput} onChange={(value) => setFeedbackInput(String(value) as QueryHistoryFeedbackFilter)} data-testid="query-history-feedback-select">
            <option value="all">{t('settings.queryHistory.feedbackAll')}</option>
            <option value="like">{t('settings.queryHistory.feedbackLike')}</option>
            <option value="dislike">{t('settings.queryHistory.feedbackDislike')}</option>
          </select>
        </label>
        <TButton type="button" onClick={applyFilter}>{t('settings.usage.apply')}</TButton>
      </div>

      {loading ? (
        <Status>{t('common.loading')}</Status>
      ) : loadError ? (
        <div role="alert" className="flex flex-wrap items-center gap-2">
          <Status tone="error">{loadError}</Status>
          <TButton type="button" onClick={() => void load(page, filter)}>{t('common.retry')}</TButton>
        </div>
      ) : rows.length === 0 ? (
        <Status>{t('common.empty')}</Status>
      ) : (
        <div className="overflow-x-auto">
          <table className={QH_TABLE} data-testid="query-history-table">
            <thead>
              <tr className="border-b border-[#e7e7ea]">
                <th className={QH_TABLE_CELL + ' font-semibold'}>{t('settings.queryHistory.colTitle')}</th>
                <th className={QH_TABLE_CELL + ' font-semibold'}>{t('settings.queryHistory.colUser')}</th>
                <th className={QH_TABLE_CELL + ' font-semibold'}>{t('settings.queryHistory.colSource')}</th>
                <th className={QH_TABLE_CELL + ' font-semibold'}>{t('settings.queryHistory.colEngine')}</th>
                <th className={QH_TABLE_CELL + ' font-semibold'}>{t('settings.queryHistory.colCreated')}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const created = queryHistoryDateParts(row.created_at, locale);
                return (
                  <tr
                    key={row.id}
                    tabIndex={0}
                    className="cursor-pointer outline-none hover:bg-[rgba(7,192,95,0.05)] focus:bg-[rgba(7,192,95,0.05)]"
                    onClick={() => { void openSnapshot(row); }}
                    onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); void openSnapshot(row); } }}
                  >
                    <td className={QH_TABLE_CELL + ' max-w-[280px] truncate'}>{row.title}</td>
                    <td className={QH_TABLE_CELL + ' font-mono text-xs'}>{row.user_id ?? '—'}</td>
                    <td className={QH_TABLE_CELL}>{sessionSourceText(row)}</td>
                    <td className={QH_TABLE_CELL}>{row.engine_type ?? '—'}</td>
                    <td className={QH_TABLE_CELL}>
                      <div className="flex flex-col gap-0.5">
                        <span>{created.date}</span>
                        <span className="text-[11px] text-[#8a96a8]">{created.time}</span>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <TButton type="button" onClick={() => setPage(prevPage(page))} disabled={loading || !canGoPrevPage(page)}>{t('settings.queryHistory.prevPage')}</TButton>
          <span className="text-[13px] text-[rgba(23,26,29,0.6)]" data-testid="query-history-page-indicator">{t('settings.queryHistory.pageIndicator', { page, totalPages })}</span>
          <TButton type="button" onClick={() => setPage(nextPage(page, totalPages))} disabled={loading || !canGoNextPage(page, totalPages)}>{t('settings.queryHistory.nextPage')}</TButton>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {exportPhase === 'done' ? (
            <>
              <TButton type="button" onClick={() => void downloadExport()} data-testid="query-history-export-download">{t('settings.queryHistory.exportDownload')}</TButton>
              <TButton type="button" onClick={resetExport}>{t('settings.queryHistory.exportReset')}</TButton>
            </>
          ) : exportPhase === 'failed' ? (
            <>
              <TButton type="button" onClick={() => void startExport()}>{t('settings.queryHistory.exportRetry')}</TButton>
              <TButton type="button" onClick={resetExport}>{t('settings.queryHistory.exportReset')}</TButton>
            </>
          ) : (
            <TButton type="button" onClick={() => void startExport()} disabled={exportPhase === 'starting' || exportPhase === 'polling'} data-testid="query-history-export-start">
              {exportPhase === 'starting' || exportPhase === 'polling' ? t('settings.queryHistory.exportPending') : t('settings.queryHistory.exportButton')}
            </TButton>
          )}
          {exportError ? <Status tone="error" >{exportError}</Status> : null}
        </div>
      </div>

      {/* Drawer opens only once the snapshot landed; loading shows inline
          below the table, errors show inline with a retry (empty ≠ failed). */}
      {snapshot ? (
        <QueryHistorySnapshotDrawer
          snapshot={snapshot}
          locale={locale}
          t={t}
          onClose={() => { setSnapshotSession(null); setSnapshot(null); setSnapshotError(null); }}
        />
      ) : null}
      {snapshotSession && snapshotLoading ? <Status>{t('common.loading')}</Status> : null}
      {snapshotSession && snapshotError ? (
        <div role="alert" className="flex flex-wrap items-center gap-2">
          <Status tone="error">{snapshotError}</Status>
          <TButton type="button" onClick={() => { void openSnapshot(snapshotSession); }}>{t('common.retry')}</TButton>
        </div>
      ) : null}
    </div>
  );
}
