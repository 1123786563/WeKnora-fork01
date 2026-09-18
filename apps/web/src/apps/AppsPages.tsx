import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Badge, Button, Card, Status, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@weknora/ui';
import { actionControls, appDigest, appErrorMessage, appRisk, appRows, appShort, appStatus, authorizationStatus, installationState, type AppRow } from './model.ts';
import { navigate } from '../platform/navigation.ts';

type AppMode = 'catalog' | 'connections' | 'authorization' | 'action';
type Props = { client: WeKnoraClient; mode: AppMode; id?: string; role?: string };
// zh copy mirrors frontend/src/i18n/locales/zh-CN.ts apps.* byte-exactly
// (Vue AppsView/ConnectionsView/AuthorizationView/ActionView baselines).
const copy = {
  catalog: {
    title: '应用目录',
    description: '当前空间经评审发布的动作及其权限范围；下方为本空间已安装的应用版本。界面不展示运行地址、密钥引用或内部别名。',
    empty: '暂无可用动作',
    installed: '已安装应用',
    installedEmpty: '暂无已安装应用',
    published: '已发布',
    unpublished: '未发布',
    colAction: '动作', colApp: '应用', colVersion: '版本', colProvider: '提供方', colRisk: '风险',
    colPermissions: '所需权限', colSchemaDigest: 'Schema 指纹', colPublished: '发布状态',
    colState: '状态', colScopes: '权限范围',
  },
  connections: {
    title: '应用连接',
    description: '当前空间的应用连接：区分个人与空间连接及其账号归属。界面不展示运行地址、密钥引用或内部别名。',
    empty: '暂无连接',
    memberCannotManage: '当前角色无法管理连接（需要空间所有者或管理员）。',
    colId: '连接', colKind: '类型', colAccount: '账号归属', colState: '状态', colActions: '操作',
    kindPersonal: '个人', kindSpace: '空间', accountSpace: '空间共享',
    startAuthorization: '授权', revoke: '断开',
    revokeConfirmContent: '断开后本空间立即失去该连接授权；远端清理可能仍在后台进行。确定断开吗？',
    remoteCleanupNote: '本地已断开；远端清理由后台异步完成',
  },
  authorization: {
    title: '授权状态',
    description: '轮询本地授权记录，等待外部授权完成。',
    attemptLabel: '授权记录', connectionLabel: '连接', statusLabel: '状态', expiresLabel: '过期时间',
    noUrlGuidance: '此部署不返回外部授权链接：请在 open-connector 控制面发出的通知中完成外部授权；完成后本页会自动更新。',
    back: '返回连接列表',
  },
  action: {
    title: '动作审批',
    description: '以下为服务器冻结的调用快照（账号、目标、参数）；审批即绑定该快照。',
    accountLabel: '账号（连接）', targetLabel: '目标', riskLabel: '风险', stateLabel: '状态',
    digestLabel: '内容指纹', fenceLabel: '版本围栏', argsLabel: '参数',
    approve: '批准', execute: '执行',
    noResendHint: '结果待核对期间不支持重发；请等待提供方查询结果。',
    memberCannotApprove: '当前角色无法审批或执行动作（需要空间所有者或管理员）。',
  },
} as const;

function isAbortError(cause: unknown): boolean { const error = cause as { name?: string; code?: string }; return error?.name === 'AbortError' || error?.name === 'CanceledError' || error?.code === 'ERR_CANCELED'; }
function responseRecord(value: unknown): AppRow { const root = value && typeof value === 'object' ? value as AppRow : {}; const data = root.data; return data && typeof data === 'object' && !Array.isArray(data) ? data as AppRow : root; }

function PageFrame({ title, description, loading, onReload, children }: { title: string; description: string; loading: boolean; onReload: () => void; children: ReactNode }) {
  return <main className="wk-page flex h-full flex-col gap-5 overflow-y-auto p-6"><header className="flex items-start justify-between gap-4"><div><h1 className="m-0 text-xl font-semibold text-ink">{title}</h1><p className="m-0 mt-1 text-sm text-muted">{description}</p></div><Button size="small" onClick={onReload} loading={loading}>刷新</Button></header>{loading ? <div role="status"><Status>加载中…</Status></div> : null}{children}</main>;
}

function CatalogTable({ rows, empty }: { rows: AppRow[]; empty: string }) {
  const fields = [['action_id', copy.catalog.colAction], ['app_id', copy.catalog.colApp], ['app_version', copy.catalog.colVersion], ['provider', copy.catalog.colProvider], ['risk', copy.catalog.colRisk], ['required_scopes', copy.catalog.colPermissions], ['schema_digest', copy.catalog.colSchemaDigest], ['published', copy.catalog.colPublished]] as const;
  return <Table><TableHead><TableRow>{fields.map(([, title]) => <TableHeader key={title}>{title}</TableHeader>)}</TableRow></TableHead><TableBody>{rows.length === 0 ? <TableRow><TableCell colSpan={fields.length}>{empty}</TableCell></TableRow> : rows.map((row, index) => <TableRow key={String(row.action_id ?? index)}>{fields.map(([field]) => { const value = row[field]; const content = field === 'required_scopes' && Array.isArray(value) ? value.join(', ') || '—' : field === 'risk' ? (() => { const risk = appRisk(value); return <Badge tone={risk.tone}>{risk.label}</Badge>; })() : field === 'schema_digest' ? appDigest(value) : field === 'published' ? <Badge tone={value ? 'success' : 'neutral'}>{value ? copy.catalog.published : copy.catalog.unpublished}</Badge> : appShort(value); return <TableCell key={field} title={String(value ?? '')}>{content}</TableCell>; })}</TableRow>)}</TableBody></Table>;
}
function InstallationsTable({ rows, empty }: { rows: AppRow[]; empty: string }) {
  const fields = [['app_key', copy.catalog.colApp], ['version', copy.catalog.colVersion], ['state', copy.catalog.colState], ['scopes', copy.catalog.colScopes]] as const;
  return <Table><TableHead><TableRow>{fields.map(([, title]) => <TableHeader key={title}>{title}</TableHeader>)}</TableRow></TableHead><TableBody>{rows.length === 0 ? <TableRow><TableCell colSpan={fields.length}>{empty}</TableCell></TableRow> : rows.map((row, index) => <TableRow key={String(row.id ?? index)}>{fields.map(([field]) => { const value = row[field]; const content = field === 'scopes' && Array.isArray(value) ? value.join(', ') || '—' : field === 'state' ? (() => { const state = installationState(value); return <Badge tone={state.tone}>{state.label}</Badge>; })() : appStatus(value); return <TableCell key={field}>{content}</TableCell>; })}</TableRow>)}</TableBody></Table>;
}

function CatalogPage({ client }: { client: WeKnoraClient }) {
  const [catalog, setCatalog] = useState<AppRow[]>([]); const [installed, setInstalled] = useState<AppRow[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState(''); const generation = useRef(0); const request = useRef<AbortController | null>(null);
  const load = async () => { request.current?.abort(); const run = ++generation.current; const controller = new AbortController(); request.current = controller; setLoading(true); setError(''); try { const [catalogResponse, installedResponse] = await Promise.all([client.request({ method: 'GET', path: '/api/v1/apps/catalog', signal: controller.signal }), client.request({ method: 'GET', path: '/api/v1/apps/installations', signal: controller.signal })]); if (run !== generation.current) return; setCatalog(appRows(catalogResponse)); setInstalled(appRows(installedResponse)); } catch (cause) { if (run === generation.current && !isAbortError(cause)) { setCatalog([]); setInstalled([]); setError(appErrorMessage(cause)); } } finally { if (run === generation.current) { setLoading(false); request.current = null; } } };
  useEffect(() => { void load(); return () => { generation.current += 1; request.current?.abort(); }; }, [client]);
  return <PageFrame title={copy.catalog.title} description={copy.catalog.description} loading={loading} onReload={() => void load()}>{error ? <Status tone="error">{error}</Status> : null}<Card><CatalogTable rows={catalog} empty={copy.catalog.empty} /></Card><Card><h2 className="m-0 mb-3 text-base font-semibold text-ink">{copy.catalog.installed}</h2><InstallationsTable rows={installed} empty={copy.catalog.installedEmpty} /></Card></PageFrame>;
}

function ConnectionsPage({ client, role }: { client: WeKnoraClient; role?: string }) {
  const [data, setData] = useState<AppRow[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState(''); const [busy, setBusy] = useState(''); const generation = useRef(0); const request = useRef<AbortController | null>(null); const canManage = role === 'owner' || role === 'admin';
  const load = async () => { request.current?.abort(); const run = ++generation.current; const controller = new AbortController(); request.current = controller; setLoading(true); setError(''); try { const value = await client.request({ method: 'GET', path: '/api/v1/apps/connections', signal: controller.signal }); if (run === generation.current) setData(appRows(value)); } catch (cause) { if (run === generation.current && !isAbortError(cause)) { setData([]); setError(appErrorMessage(cause)); } } finally { if (run === generation.current) { setLoading(false); request.current = null; } } };
  useEffect(() => { void load(); return () => { generation.current += 1; request.current?.abort(); }; }, [client]);
  const startAuthorization = async (row: AppRow) => { if (!canManage || busy) return; setBusy(String(row.id)); setError(''); try { const value = responseRecord(await client.request({ method: 'POST', path: `/api/v1/apps/connections/${encodeURIComponent(String(row.id))}/authorization-attempts`, body: {} })); const attemptId = String(value.attempt_id ?? ''); if (!attemptId) throw new Error('授权尝试未返回 ID'); navigate('/platform/apps/authorization/' + encodeURIComponent(attemptId)); } catch (cause) { setError(appErrorMessage(cause)); } finally { setBusy(''); } };
  const revoke = async (row: AppRow) => { if (!canManage || busy || !window.confirm(copy.connections.revokeConfirmContent)) return; setBusy(String(row.id)); setError(''); try { await client.request({ method: 'POST', path: `/api/v1/apps/connections/${encodeURIComponent(String(row.id))}/revoke`, body: { expected_version: row.auth_version } }); await load(); } catch (cause) { setError(appErrorMessage(cause)); } finally { setBusy(''); } };
  return <PageFrame title={copy.connections.title} description={copy.connections.description} loading={loading} onReload={() => void load()}>{error ? <Status tone="error">{error}</Status> : null}{!canManage ? <Status>{copy.connections.memberCannotManage}</Status> : null}<Card><Table><TableHead><TableRow>{[copy.connections.colId, copy.connections.colKind, copy.connections.colAccount, copy.connections.colState, copy.connections.colActions].map((header) => <TableHeader key={header}>{header}</TableHeader>)}</TableRow></TableHead><TableBody>{data.length === 0 ? <TableRow><TableCell colSpan={5}>{copy.connections.empty}</TableCell></TableRow> : data.map((row) => <TableRow key={String(row.id)}><TableCell title={String(row.id)}>{appShort(row.id)}</TableCell><TableCell>{row.kind === 'space' ? copy.connections.kindSpace : copy.connections.kindPersonal}</TableCell><TableCell>{row.kind === 'space' ? copy.connections.accountSpace : (appStatus(row.owner_id) || '—')}</TableCell><TableCell>{appStatus(row.state)}</TableCell><TableCell>{canManage && row.state === 'active' ? <><Button variant="text" size="small" loading={busy === String(row.id)} onClick={() => void startAuthorization(row)}>{copy.connections.startAuthorization}</Button><Button variant="text" size="small" loading={busy === String(row.id)} onClick={() => void revoke(row)}>{copy.connections.revoke}</Button></> : row.state === 'revoked' ? copy.connections.remoteCleanupNote : '—'}</TableCell></TableRow>)}</TableBody></Table></Card></PageFrame>;
}

function AuthorizationPage({ client, id }: { client: WeKnoraClient; id: string }) {
  const [attempt, setAttempt] = useState<AppRow>({}); const [loading, setLoading] = useState(true); const [error, setError] = useState(''); const timer = useRef<ReturnType<typeof setTimeout> | null>(null); const generation = useRef(0); const request = useRef<AbortController | null>(null);
  const poll = async (showLoading = false) => { if (!id) return; if (timer.current) { clearTimeout(timer.current); timer.current = null; } request.current?.abort(); const run = ++generation.current; const controller = new AbortController(); request.current = controller; if (showLoading) setLoading(true); setError(''); try { const value = responseRecord(await client.request({ method: 'GET', path: `/api/v1/apps/authorization-attempts/${encodeURIComponent(id)}`, signal: controller.signal })); if (run !== generation.current) return; setAttempt(value); if (authorizationStatus(value.status).poll) timer.current = setTimeout(() => void poll(), 3000); } catch (cause) { if (run === generation.current && !isAbortError(cause)) setError(appErrorMessage(cause)); } finally { if (run === generation.current) { setLoading(false); request.current = null; } } };
  useEffect(() => { void poll(true); return () => { generation.current += 1; if (timer.current) clearTimeout(timer.current); request.current?.abort(); }; }, [client, id]);
  return <PageFrame title={copy.authorization.title} description={copy.authorization.description} loading={loading} onReload={() => void poll(true)}>{error ? <Status tone="error">{error}</Status> : null}<Status>{copy.authorization.noUrlGuidance}</Status><Card><dl className="grid gap-3 text-sm"><div><dt className="font-medium text-muted">{copy.authorization.attemptLabel}</dt><dd className="m-0 font-mono">{id || '—'}</dd></div><div><dt className="font-medium text-muted">{copy.authorization.connectionLabel}</dt><dd className="m-0 font-mono">{appShort(attempt.connection_id)}</dd></div><div><dt className="font-medium text-muted">{copy.authorization.statusLabel}</dt><dd className="m-0">{appStatus(attempt.status)}</dd></div><div><dt className="font-medium text-muted">{copy.authorization.expiresLabel}</dt><dd className="m-0">{appStatus(attempt.expires_at)}</dd></div></dl></Card><Button variant="text" onClick={() => navigate('/platform/apps/connections')}>{copy.authorization.back}</Button></PageFrame>;
}

function ActionPage({ client, id, role }: { client: WeKnoraClient; id: string; role?: string }) {
  const [detail, setDetail] = useState<AppRow>({}); const [loading, setLoading] = useState(true); const [saving, setSaving] = useState(false); const [error, setError] = useState(''); const canDrive = role === 'owner' || role === 'admin'; const generation = useRef(0); const request = useRef<AbortController | null>(null);
  const load = async () => { if (!id) return; request.current?.abort(); const run = ++generation.current; const controller = new AbortController(); request.current = controller; setLoading(true); setError(''); try { const value = responseRecord(await client.request({ method: 'GET', path: `/api/v1/apps/actions/${encodeURIComponent(id)}`, signal: controller.signal })); if (run === generation.current) setDetail(value); } catch (cause) { if (run === generation.current && !isAbortError(cause)) { setDetail({}); setError(appErrorMessage(cause)); } } finally { if (run === generation.current) { setLoading(false); request.current = null; } } };
  useEffect(() => { void load(); return () => { generation.current += 1; request.current?.abort(); }; }, [client, id]);
  const action = detail.action && typeof detail.action === 'object' ? detail.action as AppRow : detail; const controls = actionControls(action.state, canDrive);
  const mutate = async (kind: 'approve' | 'execute') => { if (saving || !(kind === 'approve' ? controls.approve : controls.execute)) return; setSaving(true); setError(''); try { await client.request({ method: 'POST', path: `/api/v1/apps/actions/${encodeURIComponent(id)}/${kind}`, body: kind === 'approve' ? { digest: action.digest, expected_version: detail.expected_version } : {} }); await load(); } catch (cause) { setError(appErrorMessage(cause)); await load(); } finally { setSaving(false); } };
  const content = appStatus(action.content); let pretty = content; try { pretty = JSON.stringify(JSON.parse(content), null, 2); } catch { /* server content may be non-JSON legacy data */ }
  return <PageFrame title={copy.action.title} description={copy.action.description} loading={loading} onReload={() => void load()}>{error ? <Status tone="error">{error}</Status> : null}{!canDrive ? <Status>{copy.action.memberCannotApprove}</Status> : null}<Card><dl className="grid gap-3 text-sm"><div><dt className="font-medium text-muted">{copy.action.accountLabel}</dt><dd className="m-0 font-mono">{appStatus(action.connection_name)}</dd></div><div><dt className="font-medium text-muted">{copy.action.targetLabel}</dt><dd className="m-0 font-mono">{appStatus(action.target)}</dd></div><div><dt className="font-medium text-muted">{copy.action.riskLabel}</dt><dd className="m-0">—</dd></div><div><dt className="font-medium text-muted">{copy.action.stateLabel}</dt><dd className="m-0">{appStatus(action.state)}</dd></div><div><dt className="font-medium text-muted">{copy.action.digestLabel}</dt><dd className="m-0 font-mono">{appShort(action.digest)}</dd></div><div><dt className="font-medium text-muted">{copy.action.fenceLabel}</dt><dd className="m-0 font-mono">{appStatus(detail.expected_version)}</dd></div><div><dt className="font-medium text-muted">{copy.action.argsLabel}</dt><dd className="m-0 whitespace-pre-wrap rounded-control bg-surface-muted p-3">{pretty}</dd></div></dl><div className="mt-4 flex gap-2">{controls.approve ? <Button loading={saving} onClick={() => void mutate('approve')}>{copy.action.approve}</Button> : null}{controls.execute ? <Button loading={saving} onClick={() => void mutate('execute')}>{copy.action.execute}</Button> : null}</div>{action.state === 'unknown' ? <Status>{copy.action.noResendHint}</Status> : null}</Card></PageFrame>;
}

export function AppsPage({ client, mode, id, role }: Props) { return mode === 'catalog' ? <CatalogPage client={client} /> : mode === 'connections' ? <ConnectionsPage client={client} role={role} /> : mode === 'authorization' ? <AuthorizationPage client={client} id={id ?? ''} /> : <ActionPage client={client} id={id ?? ''} role={role} />; }
