import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { formatMessage } from '@weknora/i18n';
import { Alert as TAlert, Button as TButton, Descriptions as TDescriptions, MessagePlugin, Popconfirm as TPopconfirm, Table as TTable, Tag as TTag, Tooltip as TTooltip } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { appDigest, appErrorMessage, appRows, appShort, appStatus, type AppRow } from './model.ts';
import { pollBackoffDelayMs } from './pollBackoff.ts';
import { actionControls, type ActionViewModel } from './actionState.ts';
import { navigate } from '../platform/navigation.ts';
import { usePreferredLocale } from '../locale.ts';
import './apps-u.css';
import './apps.td.css';

type AppMode = 'catalog' | 'connections' | 'authorization' | 'action';
type Props = { client: WeKnoraClient; mode: AppMode; id?: string; role?: string };

type ToastTone = 'success' | 'warning' | 'error';

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

/* APP-9 — Vue 写操作反馈走 MessagePlugin 全局消息条（ConnectionsView.vue /
   ActionView.vue）；React 侧 tdesign-react MessagePlugin 同款命令式通道
   （main.tsx react-19-adapter 已接线）。 */
function showGlobalToast(tone: ToastTone, text: string): void {
  void MessagePlugin[tone](text);
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
  return <span className="wk-apps-1" ref={wrapper}>
    {children}
    {open ? <span className="wk-apps-2">
      <span>{content}</span>
      <span className="wk-apps-3">
        <TButton size="small" variant="text" onClick={() => setOpen(false)}>{cancelLabel}</TButton>
        <TButton size="small" variant="text" loading={busy} className="wk-apps-4" onClick={() => onConfirm()}>{confirmLabel}</TButton>
      </span>
    </span> : null}
  </span>;
}

/* APP-1 — authorization/action 两页对齐 Vue 布局（AuthorizationView.vue /
   ActionView.vue）：h2 标题 + t-alert 告警条 + t-descriptions 带边框表格 +
   t-tag；两页刷新按钮同 apps/connections 页 outline 形态（APP-3）。
   prefix 为各页 Vue SFC 根类前缀（类名 1:1）。 */
function PageHeader({ prefix, title, description, loading, onReload, refreshLabel }: { prefix: string; title: string; description: string; loading: boolean; onReload: () => void; refreshLabel: string }): ReactNode {
  return <div className={`${prefix}__header`}>
    <div className={`${prefix}__heading`}>
      <h2 className={`${prefix}__title`}>{title}</h2>
      <p className={`${prefix}__desc`}>{description}</p>
    </div>
    <TButton variant="outline" disabled={loading} aria-label={refreshLabel} onClick={onReload} icon={<TIcon name="refresh" />}>{refreshLabel}</TButton>
  </div>;
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
          /* 台账 #14：vue-next 仅内容溢出才启用固定表头（空态 th 继承白底）；
           * tdesign-react 有 maxHeight 即固定表头（th 涂灰）。空数据不传
           * maxHeight 对齐 Vue 空态 DOM/视觉。 */
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

/* ---- ConnectionsPage —— ConnectionsView.vue DOM 1:1（Task 11b，playbook §3） --
   t-table 直译（#id 等宽短 id / #kind/#state t-tag / #ops 双操作 + t-popconfirm）、
   t-alert error/info 双提示、header 刷新 t-button。 */
function ConnectionsPage({ client, role, t, showToast }: { client: WeKnoraClient; role?: string; t: (key: string, values?: Record<string, string | number>) => string; showToast: (tone: ToastTone, text: string) => void }) {
  const [data, setData] = useState<AppRow[]>([]); const [loading, setLoading] = useState(true); const [loadError, setLoadError] = useState(false); const [revokingId, setRevokingId] = useState(''); const generation = useRef(0); const request = useRef<AbortController | null>(null); const canManage = role === 'owner' || role === 'admin';
  const load = async () => { request.current?.abort(); const run = ++generation.current; const controller = new AbortController(); request.current = controller; setLoading(true); setLoadError(false); try { const value = await client.request({ method: 'GET', path: '/api/v1/apps/connections', signal: controller.signal }); if (run === generation.current) setData(appRows(value)); } catch (cause) { if (run === generation.current && !isAbortError(cause)) { setData([]); setLoadError(true); } } finally { if (run === generation.current) { setLoading(false); request.current = null; } } };
  useEffect(() => { void load(); return () => { generation.current += 1; request.current?.abort(); }; }, [client]);
  // Vue startAuthorization (ConnectionsView.vue:221-234): a missing attempt id
  // is an error toast, never a fabricated navigation (T13-F-3). APP-7 — 发起
  // 授权不触碰 revokingId（仅 revoke 自身互斥），发起期间断开按钮不被禁用。
  const startAuthorization = async (row: AppRow) => { if (!canManage) return; try { const value = responseRecord(await client.request({ method: 'POST', path: `/api/v1/apps/connections/${encodeURIComponent(String(row.id))}/authorization-attempts`, body: {} })); if (value.attempt_id) { navigate('/platform/apps/authorization/' + encodeURIComponent(String(value.attempt_id))); return; } showToast('error', t('apps.connections.startAuthorizationFailed')); } catch (cause) { showToast('error', errorMessage(cause) || t('apps.connections.startAuthorizationFailed')); } };
  // Vue revoke: echo the live auth_version (?? 1 covers a stale backend and
  // can only produce a safe 409), then reload. 409/VERSION_CONFLICT warns and
  // re-reads; local success is stated exactly, remote cleanup stays async.
  const revoke = async (row: AppRow) => { if (!canManage || revokingId) return; setRevokingId(String(row.id)); try { await client.request({ method: 'POST', path: `/api/v1/apps/connections/${encodeURIComponent(String(row.id))}/revoke`, body: { expected_version: Number(row.auth_version ?? 1) } }); showToast('success', t('apps.connections.revokeSuccess')); await load(); } catch (cause) { if (errorStatus(cause) === 409 || errorCode(cause) === 'VERSION_CONFLICT') { showToast('warning', t('apps.connections.revokeConflict')); await load(); } else { showToast('error', errorMessage(cause) || t('apps.connections.revokeFailed')); } } finally { setRevokingId(''); } };
  const kindLabel = (kind: unknown): string => { const text = String(kind ?? ''); if (text === 'personal') return t('apps.connections.kindPersonal'); if (text === 'space') return t('apps.connections.kindSpace'); return text; };
  const accountLabel = (row: AppRow): string => { if (row.kind === 'space') return t('apps.connections.accountSpace'); const owner = String(row.owner_id ?? '').trim(); return owner ? vueShortId(owner) : t('apps.connections.accountUnknown'); };
  const accountTitle = (row: AppRow): string => row.kind === 'space' ? t('apps.connections.accountSpace') : String(row.owner_id ?? '') || '';
  const stateLabel = (state: unknown): string => { const text = String(state ?? ''); if (text === 'active') return t('apps.common.stateActive'); if (text === 'revoked') return t('apps.common.stateRevoked'); return t('apps.common.stateOther', { state: text }); };
  const columns = [
    { colKey: 'id', title: t('apps.connections.colId'), width: 170, cell: ({ row }: { row: AppRow }) => <span title={String(row.id ?? '')} className="connections-view__mono">{vueShortId(row.id)}</span> },
    { colKey: 'kind', title: t('apps.connections.colKind'), width: 100, cell: ({ row }: { row: AppRow }) => <TTag theme={row.kind === 'space' ? 'primary' : 'default'} size="small">{kindLabel(row.kind)}</TTag> },
    { colKey: 'owner', title: t('apps.connections.colAccount'), ellipsis: true, cell: ({ row }: { row: AppRow }) => <span title={accountTitle(row)}>{accountLabel(row)}</span> },
    { colKey: 'state', title: t('apps.connections.colState'), width: 110, cell: ({ row }: { row: AppRow }) => <TTag theme={row.state === 'active' ? 'success' : row.state === 'revoked' ? 'danger' : 'default'} size="small">{stateLabel(row.state)}</TTag> },
    { colKey: 'ops', title: t('apps.connections.colActions'), width: 260, cell: ({ row }: { row: AppRow }) => (
      <div className="connections-view__ops">
        {canManage && row.state === 'active' ? (<>
          <TButton size="small" variant="text" aria-label={t('apps.connections.startAuthorization')} onClick={() => void startAuthorization(row)}>{t('apps.connections.startAuthorization')}</TButton>
          <TPopconfirm
            content={t('apps.connections.revokeConfirmContent')}
            confirmBtn={{ content: t('apps.connections.revoke'), theme: 'danger', loading: revokingId === String(row.id) }}
            cancelBtn={{ content: t('apps.common.cancel'), theme: 'default' }}
            onConfirm={() => void revoke(row)}
          >
            <TButton size="small" variant="text" theme="danger" disabled={revokingId !== ''} aria-label={t('apps.connections.revoke')}>{t('apps.connections.revoke')}</TButton>
          </TPopconfirm>
        </>) : row.state === 'revoked' ? <span className="connections-view__cleanup-note">{t('apps.connections.remoteCleanupNote')}</span> : null}
      </div>
    ) },
  ];
  return (
    <div className="connections-view">
      <div className="connections-view__header">
        <div className="connections-view__heading">
          <h2 className="connections-view__title">{t('apps.connections.title')}</h2>
          <p className="connections-view__desc">{t('apps.connections.description')}</p>
        </div>
        <TButton variant="outline" disabled={loading} aria-label={t('apps.connections.refresh')} onClick={() => void load()} icon={<TIcon name="refresh" />}>
          {t('apps.connections.refresh')}
        </TButton>
      </div>
      {loadError ? <TAlert theme="error" message={t('apps.connections.loadFailed')} className="connections-view__error" /> : null}
      {!canManage ? <TAlert theme="info" message={t('apps.connections.memberCannotManage')} className="connections-view__error" /> : null}
      <TTable rowKey="id" data={data} columns={columns} loading={loading} empty={t('apps.connections.empty')} hover />
    </div>
  );
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
  const statusTheme: 'success' | 'danger' | 'warning' | 'default' = status === 'active' ? 'success' : status === 'failed' || status === 'expired' || status === 'revoked' ? 'danger' : polling ? 'warning' : 'default';
  // APP-2 — Vue statusLabel computed 对空状态不做提前返回：key 未命中即落
  // status.other 插值（空 state → 「状态：」），从不显示破折号。
  const statusLabel = (() => { const label = t(`apps.authorization.status.${status}`); return label === `apps.authorization.status.${status}` ? t('apps.authorization.status.other', { state: status }) : label; })();
  return <div className="authorization-view">
    <PageHeader prefix="authorization-view" title={t('apps.authorization.title')} description={t('apps.authorization.description')} loading={loading} onReload={() => void pollOnce()} refreshLabel={t('apps.authorization.refresh')} />
    {loadError ? <TAlert theme="error" message={t('apps.authorization.loadFailed')} className="authorization-view__alert" /> : null}
    {/* T13-F-3 (binding): no authorization URL exists on this path — state the
        control-plane guidance, never a fabricated or linked URL. */}
    <TAlert theme="info" message={t('apps.authorization.noUrlGuidance')} className="authorization-view__alert" />
    <div className="authorization-view__card" role="status" aria-live={polled ? 'polite' : 'off'}>
      <TDescriptions column={1} bordered size="medium" colon items={[
        { label: t('apps.authorization.attemptLabel'), content: <span className="authorization-view__mono">{id || '—'}</span> },
        { label: t('apps.authorization.connectionLabel'), content: attempt.connection_id ? <span title={String(attempt.connection_id)} className="authorization-view__mono">{vueShortId(attempt.connection_id)}</span> : <span>—</span> },
        { label: t('apps.authorization.statusLabel'), content: <TTag theme={statusTheme} size="small">{statusLabel}</TTag> },
        { label: t('apps.authorization.expiresLabel'), content: formatTime(attempt.expires_at) },
      ]} />
      {polling ? <p className="authorization-view__hint">{t('apps.authorization.pollingHint')}</p> : succeeded ? <p className="authorization-view__hint authorization-view__hint--ok">{t('apps.authorization.completedHint')}</p> : null}
    </div>
    <TButton variant="text" aria-label={t('apps.authorization.back')} onClick={() => navigate('/platform/apps/connections')} icon={<TIcon name="arrow-left" />}>{t('apps.authorization.back')}</TButton>
  </div>;
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
  // APP-8 — Vue ActionView stateTheme: unknown → warning（结果待核对），非红。
  const stateTheme: 'success' | 'warning' | 'danger' | 'default' = state === 'succeeded' ? 'success' : state === 'failed' ? 'danger' : state === 'awaiting_approval' || state === 'unknown' ? 'warning' : 'default';
  const riskTheme: 'success' | 'warning' | 'danger' | 'default' = riskValue === 'read' ? 'success' : riskValue === 'write' ? 'warning' : riskValue === 'send' || riskValue === 'delete' ? 'danger' : 'default';
  const prettyArgs = (() => { const raw = String(action.content ?? ''); if (!raw) return '—'; try { return JSON.stringify(JSON.parse(raw), null, 2); } catch { return raw; } })();
  const shortText = (value: string): string => value && value.length > 16 ? value.slice(0, 16) + '…' : value || '—';
  return <div className="action-view">
    <PageHeader prefix="action-view" title={t('apps.actions.title')} description={t('apps.actions.description')} loading={loading} onReload={() => void reload()} refreshLabel={t('apps.actions.refresh')} />
    {loadError ? <TAlert theme="error" message={notFound ? t('apps.actions.notFound') : t('apps.actions.loadFailed')} className="action-view__alert" /> : null}
    {hasAction ? <>
      <div className="action-view__card">
        <TDescriptions column={1} bordered size="medium" colon items={[
          { label: t('apps.actions.accountLabel'), content: <span className="action-view__mono">{appStatus(action.connection_name)}</span> },
          { label: t('apps.actions.targetLabel'), content: <span className="action-view__mono">{appStatus(action.target)}</span> },
          { label: t('apps.actions.riskLabel'), content: riskLabel ? <TTag theme={riskTheme} size="small">{riskLabel}</TTag> : <TTooltip content={t('apps.actions.riskUnknownHint')}><span className="action-view__risk-missing">—</span></TTooltip> },
          { label: t('apps.actions.stateLabel'), content: <TTag theme={stateTheme} size="small">{stateLabel}</TTag> },
          { label: t('apps.actions.digestLabel'), content: <span className="action-view__mono" title={digest}>{shortText(digest)}</span> },
          { label: t('apps.actions.fenceLabel'), content: <span className="action-view__mono">{appStatus(detail?.expected_version, '0')}</span> },
          { label: t('apps.actions.argsLabel'), content: <pre className="action-view__args" tabIndex={0}>{prettyArgs}</pre> },
        ]} />
      </div>
      <div className="action-view__controls">
        {controls.approve ? <TButton loading={approving} onClick={() => void mutate('approve')}>{t('apps.actions.approve')}</TButton> : null}
        {controls.execute ? <TButton loading={submitting} onClick={() => void mutate('execute')}>{t('apps.actions.execute')}</TButton> : null}
        {state === 'unknown' ? <p>{t('apps.actions.unknown')}</p> : null}
      </div>
      {state === 'unknown' ? <p className="action-view__hint">{t('apps.actions.noResendHint')}</p> : null}
      {!canDrive ? <TAlert theme="info" message={t('apps.actions.memberCannotApprove')} className="action-view__alert" /> : null}
    </> : null}
  </div>;
}

export function AppsPage({ client, mode, id, role }: Props) {
  const { t } = useAppsCopy();
  // APP-9 — Vue MessagePlugin 全局消息条（写操作反馈不再用页内局部 Toast）。
  const showToast = (tone: ToastTone, text: string) => showGlobalToast(tone, text);
  return <>{mode === 'catalog' ? <CatalogPage client={client} t={t} /> : mode === 'connections' ? <ConnectionsPage client={client} role={role} t={t} showToast={showToast} /> : mode === 'authorization' ? <AuthorizationPage client={client} id={id ?? ''} t={t} /> : <ActionPage client={client} id={id ?? ''} role={role} t={t} showToast={showToast} />}</>;
}
