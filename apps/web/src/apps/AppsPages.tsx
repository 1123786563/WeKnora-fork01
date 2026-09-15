import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@weknora/ui';

import { appErrorMessage, appRows, appShort, appStatus, type AppRow } from './model.ts';

type AppMode = 'catalog' | 'connections' | 'authorization' | 'action';
type Props = { client: WeKnoraClient; mode: AppMode; id?: string; role?: string };
const copy = {
  catalog: { title: '应用目录', description: '查看当前工作空间可用的已审核应用动作与已安装版本。', empty: '暂无可用应用动作', installed: '已安装应用', installedEmpty: '暂无已安装应用' },
  connections: { title: '应用连接', description: '管理个人与工作空间级应用连接。', empty: '暂无应用连接' },
  authorization: { title: '应用授权', description: '等待本次授权尝试完成。此部署不会在此页面伪造授权地址。' },
  action: { title: '应用动作审批', description: '审批并执行一个绑定到当前快照的应用动作。' },
} as const;

function PageFrame({ title, description, loading, onReload, children }: { title: string; description: string; loading: boolean; onReload: () => void; children: ReactNode }) {
  return <main className="wk-page flex h-full flex-col gap-5 overflow-y-auto p-6"><header className="flex items-start justify-between gap-4"><div><h1 className="m-0 text-xl font-semibold text-ink">{title}</h1><p className="m-0 mt-1 text-sm text-muted">{description}</p></div><Button size="small" onClick={onReload} loading={loading}>刷新</Button></header>{loading ? <div role="status"><Status>加载中…</Status></div> : null}{children}</main>;
}

function AppTable({ headers, data, empty }: { headers: string[]; data: AppRow[]; empty: string }) {
  return <Table><TableHead><TableRow>{headers.map((header) => <TableHeader key={header}>{header}</TableHeader>)}</TableRow></TableHead><TableBody>{data.length === 0 ? <TableRow><TableCell colSpan={headers.length}>{empty}</TableCell></TableRow> : data.map((row, index) => { const values = Object.values(row); return <TableRow key={String(row.id ?? row.action_id ?? index)}>{headers.map((header, cell) => <TableCell key={header} title={String(values[cell] ?? '')}>{appShort(values[cell])}</TableCell>)}</TableRow>; })}</TableBody></Table>;
}

function CatalogPage({ client }: { client: WeKnoraClient }) {
  const [catalog, setCatalog] = useState<AppRow[]>([]); const [installed, setInstalled] = useState<AppRow[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState('');
  const load = async () => { setLoading(true); setError(''); try { const [catalogResponse, installedResponse] = await Promise.all([client.request({ method: 'GET', path: '/api/v1/apps/catalog' }), client.request({ method: 'GET', path: '/api/v1/apps/installations' })]); setCatalog(appRows(catalogResponse)); setInstalled(appRows(installedResponse)); } catch (cause) { setCatalog([]); setInstalled([]); setError(appErrorMessage(cause)); } finally { setLoading(false); } };
  useEffect(() => { void load(); }, [client]);
  return <PageFrame title={copy.catalog.title} description={copy.catalog.description} loading={loading} onReload={() => void load()}>{error ? <Status tone="error">{error}</Status> : null}<Card><AppTable headers={['动作', '应用', '版本', '提供方', '风险', '权限', '摘要', '发布']} data={catalog} empty={copy.catalog.empty} /></Card><Card><h2 className="m-0 mb-3 text-base font-semibold text-ink">{copy.catalog.installed}</h2><AppTable headers={['应用', '版本', '状态', '权限']} data={installed} empty={copy.catalog.installedEmpty} /></Card></PageFrame>;
}

function ConnectionsPage({ client, role }: { client: WeKnoraClient; role?: string }) {
  const [data, setData] = useState<AppRow[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState(''); const canManage = role === 'owner' || role === 'admin';
  const load = async () => { setLoading(true); setError(''); try { setData(appRows(await client.request({ method: 'GET', path: '/api/v1/apps/connections' }))); } catch (cause) { setData([]); setError(appErrorMessage(cause)); } finally { setLoading(false); } };
  useEffect(() => { void load(); }, [client]);
  const revoke = async (row: AppRow) => { if (!canManage || !window.confirm('确认撤销此应用连接？')) return; setError(''); try { await client.request({ method: 'POST', path: `/api/v1/apps/connections/${encodeURIComponent(String(row.id))}/revoke`, body: { expected_version: row.auth_version } }); await load(); } catch (cause) { setError(appErrorMessage(cause)); } };
  return <PageFrame title={copy.connections.title} description={copy.connections.description} loading={loading} onReload={() => void load()}>{error ? <Status tone="error">{error}</Status> : null}{!canManage ? <Status>当前角色只能查看应用连接。</Status> : null}<Card><Table><TableHead><TableRow>{['连接 ID', '类型', '所有者', '状态', '操作'].map((header) => <TableHeader key={header}>{header}</TableHeader>)}</TableRow></TableHead><TableBody>{data.length === 0 ? <TableRow><TableCell colSpan={5}>{copy.connections.empty}</TableCell></TableRow> : data.map((row) => <TableRow key={String(row.id)}><TableCell>{appShort(row.id)}</TableCell><TableCell>{appStatus(row.kind)}</TableCell><TableCell>{row.kind === 'space' ? '工作空间' : appShort(row.owner_id)}</TableCell><TableCell>{appStatus(row.state)}</TableCell><TableCell>{canManage && row.state === 'active' ? <Button variant="text" size="small" onClick={() => void revoke(row)}>撤销</Button> : row.state === 'revoked' ? '远端清理中' : '—'}</TableCell></TableRow>)}</TableBody></Table></Card></PageFrame>;
}

function AuthorizationPage({ client, id }: { client: WeKnoraClient; id: string }) {
  const [attempt, setAttempt] = useState<AppRow>({}); const [loading, setLoading] = useState(true); const [error, setError] = useState('');
  const poll = async (showLoading = false) => { if (showLoading) setLoading(true); setError(''); try { const value = await client.request({ method: 'GET', path: `/api/v1/apps/authorization-attempts/${encodeURIComponent(id)}` }); const root = value && typeof value === 'object' ? value as AppRow : {}; setAttempt((root.data && typeof root.data === 'object' ? root.data : root) as AppRow); } catch (cause) { setError(appErrorMessage(cause)); } finally { if (showLoading) setLoading(false); } };
  useEffect(() => { void poll(true); const timer = window.setInterval(() => void poll(), 3000); return () => window.clearInterval(timer); }, [client, id]);
  return <PageFrame title={copy.authorization.title} description={copy.authorization.description} loading={loading} onReload={() => void poll(true)}>{error ? <Status tone="error">{error}</Status> : null}<Card><dl className="grid gap-3 text-sm"><div><dt className="font-medium text-muted">尝试 ID</dt><dd className="m-0 font-mono">{id || '—'}</dd></div><div><dt className="font-medium text-muted">连接 ID</dt><dd className="m-0 font-mono">{appShort(attempt.connection_id)}</dd></div><div><dt className="font-medium text-muted">状态</dt><dd className="m-0">{appStatus(attempt.status, 'pending')}</dd></div><div><dt className="font-medium text-muted">过期时间</dt><dd className="m-0">{appStatus(attempt.expires_at)}</dd></div></dl></Card><Button variant="text" onClick={() => window.location.assign('/platform/apps/connections')}>返回连接</Button></PageFrame>;
}

function ActionPage({ client, id, role }: { client: WeKnoraClient; id: string; role?: string }) {
  const [detail, setDetail] = useState<AppRow>({}); const [loading, setLoading] = useState(true); const [saving, setSaving] = useState(false); const [error, setError] = useState(''); const canApprove = role === 'owner' || role === 'admin';
  const load = async () => { setLoading(true); setError(''); try { const value = await client.request({ method: 'GET', path: `/api/v1/apps/actions/${encodeURIComponent(id)}` }); const root = value && typeof value === 'object' ? value as AppRow : {}; setDetail((root.data && typeof root.data === 'object' ? root.data : root) as AppRow); } catch (cause) { setError(appErrorMessage(cause)); } finally { setLoading(false); } };
  useEffect(() => { void load(); }, [client, id]);
  const mutate = async (kind: 'approve' | 'execute') => { if (!canApprove || saving) return; const action = (detail.action && typeof detail.action === 'object' ? detail.action : detail) as AppRow; setSaving(true); setError(''); try { const path = kind === 'approve' ? `/api/v1/apps/actions/${encodeURIComponent(id)}/approve` : `/api/v1/apps/actions/${encodeURIComponent(id)}/execute`; const body = kind === 'approve' ? { digest: action.digest, expected_version: detail.expected_version } : {}; const value = await client.request({ method: 'POST', path, body }); const root = value && typeof value === 'object' ? value as AppRow : {}; setDetail((root.data && typeof root.data === 'object' ? root.data : root) as AppRow); } catch (cause) { setError(appErrorMessage(cause)); } finally { setSaving(false); } };
  const action = (detail.action && typeof detail.action === 'object' ? detail.action : detail) as AppRow; const status = appStatus(action.state, 'pending');
  return <PageFrame title={copy.action.title} description={copy.action.description} loading={loading} onReload={() => void load()}>{error ? <Status tone="error">{error}</Status> : null}{!canApprove ? <Status>当前角色无权审批或执行应用动作。</Status> : null}<Card><dl className="grid gap-3 text-sm"><div><dt className="font-medium text-muted">动作 ID</dt><dd className="m-0 font-mono">{id || '—'}</dd></div><div><dt className="font-medium text-muted">目标</dt><dd className="m-0">{appStatus(action.target)}</dd></div><div><dt className="font-medium text-muted">风险</dt><dd className="m-0">{appStatus(action.risk)}</dd></div><div><dt className="font-medium text-muted">状态</dt><dd className="m-0">{status}</dd></div><div><dt className="font-medium text-muted">内容</dt><dd className="m-0 whitespace-pre-wrap rounded-control bg-surface-muted p-3">{appStatus(action.content)}</dd></div></dl><div className="mt-4 flex gap-2">{canApprove && status === 'pending' ? <Button loading={saving} onClick={() => void mutate('approve')}>批准</Button> : null}{canApprove && status === 'approved' ? <Button loading={saving} onClick={() => void mutate('execute')}>执行</Button> : null}</div></Card></PageFrame>;
}

export function AppsPage({ client, mode, id, role }: Props) { return mode === 'catalog' ? <CatalogPage client={client} /> : mode === 'connections' ? <ConnectionsPage client={client} role={role} /> : mode === 'authorization' ? <AuthorizationPage client={client} id={id ?? ''} /> : <ActionPage client={client} id={id ?? ''} role={role} />; }
