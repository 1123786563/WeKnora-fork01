import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
import { Button, Card, Status, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@weknora/ui';
import { Alert as TAlert, Button as TButton, Table as TTable, Tag as TTag } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { appDigest, appErrorMessage, appRows, appShort, appStatus, type AppRow } from './model.ts';
import { pollBackoffDelayMs } from './pollBackoff.ts';
import { actionControls, type ActionViewModel } from './actionState.ts';
import { navigate } from '../platform/navigation.ts';
import { usePreferredLocale } from '../locale.ts';
import './apps.td.css';

type AppMode = 'catalog' | 'connections' | 'authorization' | 'action';
type Props = { client: WeKnoraClient; mode: AppMode; id?: string; role?: string };

type ToastTone = 'success' | 'warning' | 'error';
type ToastState = { tone: ToastTone; text: string } | null;

function isAbortError(cause: unknown): boolean {
  const error = cause as { name?: string; code?: string };
  return error?.name === 'AbortError' || error?.name === 'CanceledError' || error?.code === 'ERR_CANCELED';
}
function responseRecord(value: unknown): AppRow {
  const root = value && typeof value === 'object' ? value as AppRow : {};
  const data = root.data;
  return data && typeof data === 'object' && !Array.isArray(data) ? data as AppRow : root;
}
/** ApiError parity for the Vue err.status / err.error.code checks. */
function errorStatus(cause: unknown): number | undefined {
  return (cause as { status?: number } | null)?.status;
}
function errorCode(cause: unknown): string {
  const nested = (cause as { code?: string } | null)?.code;
  return typeof nested === 'string' ? nested : '';
}
function errorMessage(cause: unknown): string {
  const message = (cause as { message?: unknown } | null)?.message;
  return typeof message === 'string' ? message : '';
}

/* Toast mirrors the local-toast pattern used by OrganizationsPage (Vue shows
   MessagePlugin toasts on these flows). */
function Toast({ toast }: { toast: ToastState }) {
  if (!toast) return null;
  return <div role="status" aria-live="polite" className={`fixed left-1/2 top-[24px] z-[10060] -translate-x-1/2 rounded-[8px] px-[14px] py-[8px] text-[13px] text-white shadow-[0_4px_12px_rgba(0,0,0,0.2)] ${toast.tone === 'success' ? 'bg-[rgba(7,192,95,0.9)]' : toast.tone === 'warning' ? 'bg-[rgba(250,173,20,0.92)]' : 'bg-[rgba(213,73,65,0.92)]'}`}>{toast.text}</div>;
}

/* Vue t-icon refresh glyph (tdesign-icons-vue-next refresh, 24 viewBox). */
function IconRefresh({ size = 14 }: { size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true"><path d="M21.448 13C20.9483 17.7767 16.909 21.5 12 21.5C8.18227 21.5 4.89052 19.248 3.38065 16M2.5 20.5V15.5H5.5M2.55176 11C3.05145 6.22334 7.09079 2.5 11.9998 2.5C15.8175 2.5 19.1092 4.75197 20.6191 8M21.4998 3.5V8.5H18.4998" stroke="currentColor" strokeWidth="2" strokeLinecap="square" /></svg>;
}

/* Vue t-tag (variant dark, size small): solid 20px chip, 12px text, radius 3. */
function Tag({ theme = 'default', children }: { theme?: 'success' | 'warning' | 'danger' | 'primary' | 'default'; children: ReactNode }) {
  const palette = theme === 'success' ? 'bg-[#00a870] text-white'
    : theme === 'warning' ? 'bg-[#e37318] text-white'
    : theme === 'danger' ? 'bg-[#d54941] text-white'
    : theme === 'primary' ? 'bg-[#0052d9] text-white'
    : 'bg-[#e7e7e7] text-[rgba(0,0,0,0.9)]';
  return <span className={`inline-flex box-border h-5 shrink-0 items-center rounded-[3px] border border-transparent px-1 text-xs leading-5 ${palette}`}>{children}</span>;
}

/* Vue TDesign table cell metrics (AppsView.vue/ConnectionsView.vue + t-table):
   th 14px/22px py12 px16, box 47 with its 1px #dcdcdc bottom rule; td 14px/22px
   py12 px16 with a 1px #dcdcdc bottom rule per row; TDesign uses a FIXED
   layout so Vue's column widths are literal border-box widths and the
   widthless columns split the rest. */
const TDESIGN_TABLE = 'table-fixed [&_thead]:border-[#dcdcdc] [&_th]:box-border [&_th]:h-[47px] [&_th]:px-4 [&_th]:py-0 [&_th]:text-[14px] [&_th]:leading-[22px] [&_th]:font-normal [&_th]:align-middle [&_th]:text-[rgba(0,0,0,0.4)] [&_td]:box-border [&_td]:border-b [&_td]:border-[#dcdcdc] [&_td]:px-4 [&_td]:py-3 [&_td]:text-[14px] [&_td]:leading-[22px] [&_td]:align-middle [&_td]:text-[rgba(0,0,0,0.9)]';

/* 留守段（authorization/action 两页）沿用旧栈 @weknora/ui 表格的空态单元格：
   Vue t-table empty prop 的手排版（120px 居中占位）。 */
function EmptyCell({ colSpan, text }: { colSpan: number; text: string }) {
  return <TableCell colSpan={colSpan} className="text-center" style={{ padding: 0, borderBottom: 'none' }}><div className="flex h-[120px] items-center justify-center text-[14px] leading-[22px] text-[rgba(0,0,0,0.26)]">{text}</div></TableCell>;
}

/* TDesign t-popconfirm counterpart: an anchored confirm bubble with a danger
   confirm button (loading supported) and a default cancel button. */
function Popconfirm({ content, confirmLabel, cancelLabel, busy, onConfirm, children }: { content: string; confirmLabel: string; cancelLabel: string; busy: boolean; onConfirm: () => void; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const wrapper = useRef<HTMLSpanElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => {
      if (wrapper.current && !wrapper.current.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, [open]);
  return <span className="relative inline-flex" ref={wrapper}>
    {children}
    {open ? <span className="absolute right-0 top-full z-[60] mt-[6px] flex w-[240px] flex-col gap-[8px] rounded-[8px] border border-[#e7e7ea] bg-white p-[12px] text-[13px] text-[rgba(23,26,29,0.92)] shadow-[0_4px_16px_rgba(0,0,0,0.12)]">
      <span>{content}</span>
      <span className="flex items-center justify-end gap-[8px]">
        <Button size="small" variant="text" onClick={() => setOpen(false)}>{cancelLabel}</Button>
        <Button size="small" variant="text" loading={busy} className="text-[#d54941]" onClick={() => onConfirm()}>{confirmLabel}</Button>
      </span>
    </span> : null}
  </span>;
}

/* 留守段 frame（authorization/action 两页，R490 React 端口自持）：
   apps/apps-connections 两页已迁 Vue DOM（.apps-view/.connections-view +
   apps.td.css），此 frame 仅供未扫描的两页沿用旧布局。 */
function PageFrame({ title, description, loading, onReload, refreshLabel, loadingLabel, gapClass = 'gap-5', children }: { title: string; description: string; loading: boolean; onReload: () => void; refreshLabel: string; loadingLabel: string; gapClass?: string; children: ReactNode }) {
  return <main className={`wk-page flex h-full flex-col overflow-y-auto p-6 [-webkit-font-smoothing:antialiased] [-moz-osx-font-smoothing:grayscale] ${gapClass}`}><header className="flex items-start justify-between gap-4"><div><h1 className="m-0 text-xl font-semibold leading-[28px] text-[rgba(0,0,0,0.9)]">{title}</h1><p className="m-0 mt-1 max-w-[640px] text-[13px] leading-[18px] text-[rgba(0,0,0,0.6)]">{description}</p></div><Button aria-label={refreshLabel} disabled={loading} onClick={onReload} loading={loading} className="h-8 shrink-0 gap-[8px] rounded-[3px] border-[#dcdcdc] px-[15px] text-[14px] leading-[22px] text-[rgba(0,0,0,0.9)]"><IconRefresh size={14} />{refreshLabel}</Button></header>{loading ? <div role="status"><Status>{loadingLabel}</Status></div> : null}{children}</main>;
}

function useAppsCopy() {
  const locale = usePreferredLocale();
  const t = (key: string, values?: Record<string, string | number>): string => formatMessage(locale, key, values);
  return { t };
}
type AppsTranslate = (key: string, values?: Record<string, string | number>) => string;

/* ---- CatalogPage —— AppsView.vue DOM 1:1（Task 11b，playbook §3） ----------
   t-table 直译（columns + cell 渲染函数）、t-tag 风险/发布态、t-alert 加载失败、
   header 刷新 t-button（variant outline）。 */
function catalogColumns(t: AppsTranslate) {
  return [
    { colKey: 'action_id', title: t('apps.catalog.colAction'), ellipsis: true, cell: ({ row }: { row: AppRow }) => appShort(row.action_id) },
    { colKey: 'app_id', title: t('apps.catalog.colApp'), ellipsis: true, cell: ({ row }: { row: AppRow }) => appShort(row.app_id) },
    { colKey: 'app_version', title: t('apps.catalog.colVersion'), width: 110, cell: ({ row }: { row: AppRow }) => appShort(row.app_version) },
    { colKey: 'provider', title: t('apps.catalog.colProvider'), width: 120, ellipsis: true, cell: ({ row }: { row: AppRow }) => appShort(row.provider) },
    { colKey: 'risk', title: t('apps.catalog.colRisk'), width: 100, cell: ({ row }: { row: AppRow }) => { const risk = String(row.risk ?? '').trim(); return risk ? <TTag theme={riskThemeOf(risk)} size="small">{riskLabelOf(risk, t)}</TTag> : '—'; } },
    { colKey: 'required_scopes', title: t('apps.catalog.colPermissions'), ellipsis: true, cell: ({ row }: { row: AppRow }) => <span>{Array.isArray(row.required_scopes) && row.required_scopes.length ? row.required_scopes.join(', ') : '—'}</span> },
    { colKey: 'schema_digest', title: t('apps.catalog.colSchemaDigest'), width: 150, cell: ({ row }: { row: AppRow }) => <span title={String(row.schema_digest ?? '')}>{appDigest(row.schema_digest)}</span> },
    { colKey: 'published', title: t('apps.catalog.colPublished'), width: 110, cell: ({ row }: { row: AppRow }) => row.published ? <TTag theme="success" size="small">{t('apps.catalog.published')}</TTag> : <TTag theme="default" size="small">{t('apps.catalog.unpublished')}</TTag> },
  ];
}

function installationColumns(t: AppsTranslate) {
  return [
    { colKey: 'app_key', title: t('apps.catalog.colApp'), ellipsis: true, cell: ({ row }: { row: AppRow }) => appShort(row.app_key) },
    { colKey: 'version', title: t('apps.catalog.colVersion'), width: 110, cell: ({ row }: { row: AppRow }) => appShort(row.version) },
    { colKey: 'state', title: t('apps.catalog.colState'), width: 110, cell: ({ row }: { row: AppRow }) => { const state = String(row.state ?? '').trim(); const label = state === 'active' ? t('apps.common.stateActive') : state === 'disabled' ? t('apps.common.stateDisabled') : state ? t('apps.common.stateOther', { state }) : '—'; return <TTag theme={state === 'active' ? 'success' : 'default'} size="small">{label}</TTag>; } },
    { colKey: 'scopes', title: t('apps.catalog.colScopes'), ellipsis: true, cell: ({ row }: { row: AppRow }) => <span>{Array.isArray(row.scopes) && row.scopes.length ? row.scopes.join(', ') : '—'}</span> },
  ];
}

function riskThemeOf(risk: string): 'success' | 'warning' | 'danger' | 'default' {
  if (risk === 'read') return 'success';
  if (risk === 'write') return 'warning';
  if (risk === 'send' || risk === 'delete') return 'danger';
  return 'default';
}

function riskLabelOf(risk: string, t: AppsTranslate): string {
  const key = 'apps.risk.' + risk;
  const label = t(key);
  return label === key ? risk : label;
}

function CatalogPage({ client, t }: { client: WeKnoraClient; t: (key: string, values?: Record<string, string | number>) => string }) {
  const [catalog, setCatalog] = useState<AppRow[]>([]); const [installed, setInstalled] = useState<AppRow[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState(false); const generation = useRef(0); const request = useRef<AbortController | null>(null);
  const load = async () => { request.current?.abort(); const run = ++generation.current; const controller = new AbortController(); request.current = controller; setLoading(true); setError(false); try { const [catalogResponse, installedResponse] = await Promise.all([client.request({ method: 'GET', path: '/api/v1/apps/catalog', signal: controller.signal }), client.request({ method: 'GET', path: '/api/v1/apps/installations', signal: controller.signal })]); if (run !== generation.current) return; setCatalog(appRows(catalogResponse)); setInstalled(appRows(installedResponse)); } catch (cause) { if (run === generation.current && !isAbortError(cause)) { setCatalog([]); setInstalled([]); setError(true); } } finally { if (run === generation.current) { setLoading(false); request.current = null; } } };
  useEffect(() => { void load(); return () => { generation.current += 1; request.current?.abort(); }; }, [client]);
  return (
    <div className="apps-view">
      <div className="apps-view__header">
        <div className="apps-view__heading">
          <h2 className="apps-view__title">{t('apps.catalog.title')}</h2>
          <p className="apps-view__desc">{t('apps.catalog.description')}</p>
        </div>
        <TButton variant="outline" disabled={loading} aria-label={t('apps.catalog.refresh')} onClick={() => void load()} icon={<TIcon name="refresh" />}>
          {t('apps.catalog.refresh')}
        </TButton>
      </div>
      {error ? <TAlert theme="error" message={t('apps.catalog.loadFailed')} className="apps-view__error" /> : null}
      <section className="apps-view__section" aria-label={t('apps.catalog.title')}>
        <TTable
          rowKey="action_id"
          data={catalog}
          columns={catalogColumns(t)}
          loading={loading}
          empty={t('apps.catalog.empty')}
          hover
          /* vue-next 1.20.7 仅在内容溢出时启用固定表头（空态 th 继承白底）；
           * tdesign-react 1.18.3 有 maxHeight 即固定表头（th 涂灰）。空数据时
           * 不传 maxHeight 以对齐 Vue 空态 DOM/视觉（台账候选，Phase 4 归全局）。 */
          maxHeight={catalog.length ? 520 : undefined}
        />
      </section>
      <section className="apps-view__section" aria-label={t('apps.catalog.installedTitle')}>
        <h3 className="apps-view__subtitle">{t('apps.catalog.installedTitle')}</h3>
        <TTable
          rowKey="id"
          data={installed}
          columns={installationColumns(t)}
          loading={loading}
          empty={t('apps.catalog.installedEmpty')}
          hover
          maxHeight={installed.length ? 360 : undefined}
        />
      </section>
    </div>
  );
}

/* Vue shortId (ConnectionsView/AuthorizationView): 14 chars, then an ellipsis. */
function vueShortId(id: unknown): string {
  const text = String(id ?? '').trim();
  return text.length > 14 ? text.slice(0, 14) + '…' : text || '—';
}

function ConnectionsPage({ client, role, t, showToast }: { client: WeKnoraClient; role?: string; t: (key: string, values?: Record<string, string | number>) => string; showToast: (tone: ToastTone, text: string) => void }) {
  const [data, setData] = useState<AppRow[]>([]); const [loading, setLoading] = useState(true); const [loadError, setLoadError] = useState(false); const [revokingId, setRevokingId] = useState(''); const generation = useRef(0); const request = useRef<AbortController | null>(null); const canManage = role === 'owner' || role === 'admin';
  const load = async () => { request.current?.abort(); const run = ++generation.current; const controller = new AbortController(); request.current = controller; setLoading(true); setLoadError(false); try { const value = await client.request({ method: 'GET', path: '/api/v1/apps/connections', signal: controller.signal }); if (run === generation.current) setData(appRows(value)); } catch (cause) { if (run === generation.current && !isAbortError(cause)) { setData([]); setLoadError(true); } } finally { if (run === generation.current) { setLoading(false); request.current = null; } } };
  useEffect(() => { void load(); return () => { generation.current += 1; request.current?.abort(); }; }, [client]);
  // Vue startAuthorization: a missing attempt id is an error toast, never a
  // fabricated navigation (T13-F-3).
  const startAuthorization = async (row: AppRow) => { if (!canManage || revokingId) return; setRevokingId(String(row.id)); try { const value = responseRecord(await client.request({ method: 'POST', path: `/api/v1/apps/connections/${encodeURIComponent(String(row.id))}/authorization-attempts`, body: {} })); if (value.attempt_id) { navigate('/platform/apps/authorization/' + encodeURIComponent(String(value.attempt_id))); return; } showToast('error', t('apps.connections.startAuthorizationFailed')); } catch (cause) { showToast('error', errorMessage(cause) || t('apps.connections.startAuthorizationFailed')); } finally { setRevokingId(''); } };
  // Vue revoke: echo the live auth_version (?? 1 covers a stale backend and
  // can only produce a safe 409), then reload. 409/VERSION_CONFLICT warns and
  // re-reads; local success is stated exactly, remote cleanup stays async.
  const revoke = async (row: AppRow) => { if (!canManage || revokingId) return; setRevokingId(String(row.id)); try { await client.request({ method: 'POST', path: `/api/v1/apps/connections/${encodeURIComponent(String(row.id))}/revoke`, body: { expected_version: Number(row.auth_version ?? 1) } }); showToast('success', t('apps.connections.revokeSuccess')); await load(); } catch (cause) { if (errorStatus(cause) === 409 || errorCode(cause) === 'VERSION_CONFLICT') { showToast('warning', t('apps.connections.revokeConflict')); await load(); } else { showToast('error', errorMessage(cause) || t('apps.connections.revokeFailed')); } } finally { setRevokingId(''); } };
  const kindLabel = (kind: unknown): string => { const text = String(kind ?? ''); if (text === 'personal') return t('apps.connections.kindPersonal'); if (text === 'space') return t('apps.connections.kindSpace'); return text; };
  const accountLabel = (row: AppRow): string => { if (row.kind === 'space') return t('apps.connections.accountSpace'); const owner = String(row.owner_id ?? '').trim(); return owner ? vueShortId(owner) : t('apps.connections.accountUnknown'); };
  const accountTitle = (row: AppRow): string => row.kind === 'space' ? t('apps.connections.accountSpace') : String(row.owner_id ?? '') || '';
  const stateLabel = (state: unknown): string => { const text = String(state ?? ''); if (text === 'active') return t('apps.common.stateActive'); if (text === 'revoked') return t('apps.common.stateRevoked'); return t('apps.common.stateOther', { state: text }); };
  return <PageFrame title={t('apps.connections.title')} description={t('apps.connections.description')} loading={loading} onReload={() => void load()} refreshLabel={t('apps.connections.refresh')} loadingLabel={t('common.loading')} gapClass="gap-4">{loadError ? <Status tone="error">{t('apps.connections.loadFailed')}</Status> : null}{!canManage ? <Status>{t('apps.connections.memberCannotManage')}</Status> : null}<Table className={TDESIGN_TABLE}><TableHead><TableRow>{[[t('apps.connections.colId'), 170], [t('apps.connections.colKind'), 100], [t('apps.connections.colAccount'), undefined], [t('apps.connections.colState'), 110], [t('apps.connections.colActions'), 260]].map(([header, width]) => <TableHeader key={header as string} style={width ? { width } : undefined}>{header as string}</TableHeader>)}</TableRow></TableHead><TableBody>{data.length === 0 ? <TableRow><EmptyCell colSpan={5} text={t('apps.connections.empty')} /></TableRow> : data.map((row) => <TableRow key={String(row.id)}><TableCell title={String(row.id ?? '')}><span className="font-mono">{vueShortId(row.id)}</span></TableCell><TableCell><Tag theme={row.kind === 'space' ? 'primary' : 'default'}>{kindLabel(row.kind)}</Tag></TableCell><TableCell title={accountTitle(row)}>{accountLabel(row)}</TableCell><TableCell><Tag theme={row.state === 'active' ? 'success' : row.state === 'revoked' ? 'danger' : 'default'}>{stateLabel(row.state)}</Tag></TableCell><TableCell><span className="flex items-center gap-[4px]">{canManage && row.state === 'active' ? <><Button variant="text" size="small" className="h-6 min-h-6 px-2 text-[14px] text-[rgba(0,0,0,0.9)]" onClick={() => void startAuthorization(row)}>{t('apps.connections.startAuthorization')}</Button><Popconfirm content={t('apps.connections.revokeConfirmContent')} confirmLabel={t('apps.connections.revoke')} cancelLabel={t('apps.common.cancel')} busy={revokingId === String(row.id)} onConfirm={() => void revoke(row)}><Button variant="text" size="small" disabled={revokingId !== ''} className="h-6 min-h-6 px-2 text-[14px] text-[#d54941]">{t('apps.connections.revoke')}</Button></Popconfirm></> : row.state === 'revoked' ? <span className="text-[12px] text-[rgba(23,26,29,0.4)]">{t('apps.connections.remoteCleanupNote')}</span> : '—'}</span></TableCell></TableRow>)}</TableBody></Table></PageFrame>;
}

/* Vue formatTime: locale date string, em dash when absent. */
function formatTime(value: unknown): string {
  const text = String(value ?? '');
  if (!text) return '—';
  try { return new Date(text).toLocaleString(); } catch { return text; }
}

function AuthorizationPage({ client, id, t }: { client: WeKnoraClient; id: string; t: (key: string, values?: Record<string, string | number>) => string }) {
  const [attempt, setAttempt] = useState<AppRow>({}); const [loading, setLoading] = useState(true); const [loadError, setLoadError] = useState(false); const [polled, setPolled] = useState(false); const timer = useRef<ReturnType<typeof setTimeout> | null>(null); const generation = useRef(0); const request = useRef<AbortController | null>(null);
  // Vue pollFailures: consecutive failures double the next delay; a success
  // resets the counter (pollBackoff.ts).
  const pollFailures = useRef(0);
  const pollingStatuses = new Set(['pending', 'authorizing', 'verifying']);
  const status = String(attempt.status ?? '');
  const polling = pollingStatuses.has(status);
  const succeeded = status === 'active';
  const cancelTimer = () => { if (timer.current) { clearTimeout(timer.current); timer.current = null; } };
  const pollRef = useRef<() => Promise<void>>(async () => undefined);
  const scheduleNext = (isPolling: boolean, expires: unknown) => {
    if (timer.current) clearTimeout(timer.current);
    if (!isPolling) return; // terminal: stop polling, keep the last state
    // expires_at stop: an attempt stuck pending past its TTL must not poll
    // for the page's whole lifetime (T15 QF-3).
    const expiresMs = expires ? Date.parse(String(expires)) : NaN;
    if (!Number.isNaN(expiresMs) && expiresMs <= Date.now()) return;
    timer.current = setTimeout(() => { timer.current = null; void pollRef.current(); }, pollBackoffDelayMs(pollFailures.current));
  };
  const pollOnce = async () => {
    if (!id) return;
    if (timer.current) { clearTimeout(timer.current); timer.current = null; }
    request.current?.abort();
    const run = ++generation.current;
    const controller = new AbortController();
    request.current = controller;
    setLoading(true); setLoadError(false);
    try {
      const value = responseRecord(await client.request({ method: 'GET', path: `/api/v1/apps/authorization-attempts/${encodeURIComponent(id)}`, signal: controller.signal }));
      if (run !== generation.current) return;
      setAttempt(value); setPolled(true); pollFailures.current = 0;
      scheduleNext(pollingStatuses.has(String(value.status ?? '')), value.expires_at);
    } catch (cause) {
      if (run === generation.current && !isAbortError(cause)) {
        pollFailures.current += 1; setLoadError(true); setPolled(true);
      }
      if (run === generation.current) scheduleNext(pollingRefStatus(), undefined);
    } finally {
      if (run === generation.current) { setLoading(false); request.current = null; }
    }
  };
  // After a failed poll the last known status still governs whether the
  // attempt is worth another try (Vue reads attempt.value?.status).
  const pollingRefStatus = (): boolean => pollingStatuses.has(String(attempt.status ?? ''));
  pollRef.current = pollOnce;
  useEffect(() => {
    void pollOnce();
    return () => { generation.current += 1; cancelTimer(); request.current?.abort(); pollFailures.current = 0; };
  }, [client, id]);
  const statusTheme = status === 'active' ? 'success' : status === 'failed' || status === 'expired' || status === 'revoked' ? 'danger' : polling ? 'warning' : 'neutral';
  const statusLabel = (() => { if (!status) return '—'; const label = t(`apps.authorization.status.${status}`); return label === `apps.authorization.status.${status}` ? t('apps.authorization.status.other', { state: status }) : label; })();
  return <PageFrame title={t('apps.authorization.title')} description={t('apps.authorization.description')} loading={loading} onReload={() => void pollOnce()} refreshLabel={t('apps.authorization.refresh')} loadingLabel={t('common.loading')}>{loadError ? <Status tone="error">{t('apps.authorization.loadFailed')}</Status> : null}<Status>{t('apps.authorization.noUrlGuidance')}</Status><Card><div role="status" aria-live={polled ? 'polite' : 'off'} className="flex flex-col gap-[12px]"><dl className="m-0 grid gap-3 text-sm"><div><dt className="font-medium text-muted">{t('apps.authorization.attemptLabel')}</dt><dd className="m-0 font-mono">{id || '—'}</dd></div><div><dt className="font-medium text-muted">{t('apps.authorization.connectionLabel')}</dt><dd className="m-0 font-mono" title={String(attempt.connection_id ?? '')}>{attempt.connection_id ? vueShortId(attempt.connection_id) : '—'}</dd></div><div><dt className="font-medium text-muted">{t('apps.authorization.statusLabel')}</dt><dd className="m-0"><Tag theme={statusTheme as 'success' | 'danger' | 'warning' | 'default'}>{statusLabel}</Tag></dd></div><div><dt className="font-medium text-muted">{t('apps.authorization.expiresLabel')}</dt><dd className="m-0">{formatTime(attempt.expires_at)}</dd></div></dl>{polling ? <p className="m-0 text-[13px] text-[rgba(23,26,29,0.6)]">{t('apps.authorization.pollingHint')}</p> : succeeded ? <p className="m-0 text-[13px] text-[#0a7f43]">{t('apps.authorization.completedHint')}</p> : null}</div></Card><Button variant="text" onClick={() => navigate('/platform/apps/connections')}>{t('apps.authorization.back')}</Button></PageFrame>;
}

function ActionPage({ client, id, role, t, showToast }: { client: WeKnoraClient; id: string; role?: string; t: (key: string, values?: Record<string, string | number>) => string; showToast: (tone: ToastTone, text: string) => void }) {
  const [detail, setDetail] = useState<AppRow | null>(null); const [loading, setLoading] = useState(true); const [loadError, setLoadError] = useState(false); const [notFound, setNotFound] = useState(false); const [approving, setApproving] = useState(false); const [submitting, setSubmitting] = useState(false); const canDrive = role === 'owner' || role === 'admin'; const generation = useRef(0); const request = useRef<AbortController | null>(null);
  const action: AppRow = detail && detail.action && typeof detail.action === 'object' ? detail.action as AppRow : {};
  const reload = async () => {
    if (!id) return;
    request.current?.abort();
    const run = ++generation.current;
    const controller = new AbortController();
    request.current = controller;
    setLoading(true); setLoadError(false); setNotFound(false);
    try {
      const value = responseRecord(await client.request({ method: 'GET', path: `/api/v1/apps/actions/${encodeURIComponent(id)}`, signal: controller.signal }));
      if (run !== generation.current) return;
      setDetail(value); setNotFound(false);
    } catch (cause) {
      if (run !== generation.current || isAbortError(cause)) return;
      setNotFound(errorStatus(cause) === 404); setLoadError(true); setDetail(null);
    } finally {
      if (run === generation.current) { setLoading(false); request.current = null; }
    }
  };
  useEffect(() => { void reload(); return () => { generation.current += 1; request.current?.abort(); }; }, [client, id]);
  const hasAction = Boolean(detail && detail.action && typeof detail.action === 'object');
  const digest = String(action.digest ?? '');
  const viewModel: ActionViewModel | null = hasAction ? { id: String(action.id ?? id), state: String(action.state ?? ''), digest, canApprove: canDrive, canExecute: canDrive } : null;
  const controls = viewModel ? actionControls(viewModel) : { approve: false, execute: false, retry: false };
  // Vue approve/execute: bind the CURRENT snapshot (digest + fence echo), then
  // RE-READ from the server — success is never assumed from a 200 alone.
  const mutate = async (kind: 'approve' | 'execute') => {
    if (!hasAction || !(kind === 'approve' ? controls.approve : controls.execute)) return;
    const run = generation.current;
    const busy = kind === 'approve' ? setApproving : setSubmitting;
    busy(true);
    try {
      await client.request({ method: 'POST', path: `/api/v1/apps/actions/${encodeURIComponent(id)}/${kind}`, body: kind === 'approve' ? { digest: action.digest, expected_version: detail?.expected_version } : {} });
      if (run !== generation.current) return;
      await reload();
    } catch (cause) {
      if (run !== generation.current) return;
      showToast('error', errorMessage(cause) || t(kind === 'approve' ? 'apps.actions.approveFailed' : 'apps.actions.executeFailed'));
      await reload();
    } finally { busy(false); }
  };
  const riskValue = String(action.risk ?? '');
  const riskLabel = riskValue ? ((label) => label === `apps.risk.${riskValue}` ? riskValue : label)(t(`apps.risk.${riskValue}`)) : '';
  const state = String(action.state ?? '');
  const stateLabel = (() => { if (state === 'unknown') return t('apps.actions.unknown'); if (!state) return '—'; const label = t(`apps.actions.state.${state}`); return label === `apps.actions.state.${state}` ? t('apps.actions.state.other', { state }) : label; })();
  const stateTheme = state === 'succeeded' ? 'success' : state === 'failed' || state === 'unknown' ? 'danger' : state === 'awaiting_approval' ? 'warning' : 'neutral';
  const prettyArgs = (() => { const raw = String(action.content ?? ''); if (!raw) return '—'; try { return JSON.stringify(JSON.parse(raw), null, 2); } catch { return raw; } })();
  const shortText = (value: string): string => value && value.length > 16 ? value.slice(0, 16) + '…' : value || '—';
  return <PageFrame title={t('apps.actions.title')} description={t('apps.actions.description')} loading={loading} onReload={() => void reload()} refreshLabel={t('apps.actions.refresh')} loadingLabel={t('common.loading')}>{loadError ? <Status tone="error">{notFound ? t('apps.actions.notFound') : t('apps.actions.loadFailed')}</Status> : null}{hasAction ? <>
    <Card><dl className="m-0 grid gap-3 text-sm">
      <div><dt className="font-medium text-muted">{t('apps.actions.accountLabel')}</dt><dd className="m-0 font-mono">{appStatus(action.connection_name)}</dd></div>
      <div><dt className="font-medium text-muted">{t('apps.actions.targetLabel')}</dt><dd className="m-0 font-mono">{appStatus(action.target)}</dd></div>
      <div><dt className="font-medium text-muted">{t('apps.actions.riskLabel')}</dt><dd className="m-0" title={riskLabel ? undefined : t('apps.actions.riskUnknownHint')}>{riskLabel ? <Tag theme={riskValue === 'read' ? 'success' : riskValue === 'write' ? 'warning' : riskValue === 'send' || riskValue === 'delete' ? 'danger' : 'default'}>{riskLabel}</Tag> : <span className="text-[rgba(23,26,29,0.4)]">—</span>}</dd></div>
      <div><dt className="font-medium text-muted">{t('apps.actions.stateLabel')}</dt><dd className="m-0"><Tag theme={stateTheme as 'success' | 'danger' | 'warning' | 'default'}>{stateLabel}</Tag></dd></div>
      <div><dt className="font-medium text-muted">{t('apps.actions.digestLabel')}</dt><dd className="m-0 font-mono" title={digest}>{shortText(digest)}</dd></div>
      <div><dt className="font-medium text-muted">{t('apps.actions.fenceLabel')}</dt><dd className="m-0 font-mono">{appStatus(detail?.expected_version, '0')}</dd></div>
      <div><dt className="font-medium text-muted">{t('apps.actions.argsLabel')}</dt><dd className="m-0 whitespace-pre-wrap rounded-control bg-surface-muted p-3 font-mono text-[12px]" tabIndex={0}>{prettyArgs}</dd></div>
    </dl></Card>
    <div className="flex items-center gap-[12px]">{controls.approve ? <Button loading={approving} onClick={() => void mutate('approve')}>{t('apps.actions.approve')}</Button> : null}{controls.execute ? <Button loading={submitting} onClick={() => void mutate('execute')}>{t('apps.actions.execute')}</Button> : null}{state === 'unknown' ? <p className="m-0 text-[13px] text-[#ad4b00]">{t('apps.actions.unknown')}</p> : null}</div>
    {state === 'unknown' ? <p className="m-0 text-[12px] text-[rgba(23,26,29,0.6)]">{t('apps.actions.noResendHint')}</p> : null}
    {!canDrive ? <Status>{t('apps.actions.memberCannotApprove')}</Status> : null}
  </> : null}</PageFrame>;
}

export function AppsPage({ client, mode, id, role }: Props) {
  const { t } = useAppsCopy();
  const [toast, setToast] = useState<ToastState>(null);
  const showToast = (tone: ToastTone, text: string) => setToast({ tone, text });
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), 3000);
    return () => clearTimeout(timer);
  }, [toast]);
  return <><Toast toast={toast} />{mode === 'catalog' ? <CatalogPage client={client} t={t} /> : mode === 'connections' ? <ConnectionsPage client={client} role={role} t={t} showToast={showToast} /> : mode === 'authorization' ? <AuthorizationPage client={client} id={id ?? ''} t={t} /> : <ActionPage client={client} id={id ?? ''} role={role} t={t} showToast={showToast} />}</>;
}
