import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain/scope';
import { Button, Card, Status } from '@weknora/ui';
import { appsApi, type ConnectionEntry } from './api.ts';

export function ConnectionsPage({ client, scopeController, role, onNavigate }: { client: WeKnoraClient; scopeController: ReturnType<typeof createScopeController>; role?: string; onNavigate: (path: string) => void }) {
  const [rows, setRows] = useState<ConnectionEntry[]>([]); const [error, setError] = useState(''); const [busy, setBusy] = useState(''); const [reload, setReload] = useState(0); const canManage = role === 'owner' || role === 'admin';
  const load = () => { const scope = scopeController.current(); setError(''); void appsApi(client).listConnections(scope.signal).then((next) => { if (scopeController.isCurrent(scope.scope)) setRows(next); }).catch((e) => { if (!scope.signal.aborted && scopeController.isCurrent(scope.scope)) setError(e instanceof Error ? e.message : 'Unable to load connections'); }); };
  useEffect(load, [client, reload, scopeController]);
  const revoke = async (row: ConnectionEntry) => { if (!canManage || busy) return; setBusy(row.id); try { await appsApi(client).revokeConnection(row.id, row.auth_version); load(); } catch (e) { setError(e instanceof Error ? e.message : 'Unable to revoke connection'); } finally { setBusy(''); } };
  const authorize = async (row: ConnectionEntry) => { if (!canManage || busy) return; setBusy(row.id); try { const attempt = await appsApi(client).beginAuthorization(row.id); onNavigate('/platform/apps/authorization/' + encodeURIComponent(attempt.attempt_id)); } catch (e) { setError(e instanceof Error ? e.message : 'Unable to start authorization'); } finally { setBusy(''); } };
  return <main className="wk-page"><header className="wk-header"><div><p className="wk-eyebrow">Apps</p><h1>连接</h1><p className="wk-muted">个人连接和空间连接；凭据永不展示。</p></div><Button type="button" onClick={() => setReload((value) => value + 1)}>刷新</Button></header><Card>{!canManage ? <Status>当前成员不能管理连接。</Status> : null}{error ? <Status tone="error">{error}</Status> : null}{rows.length === 0 ? <Status>暂无连接。</Status> : <ul className="wk-list">{rows.map((row) => <li key={row.id}><strong>{row.kind === 'space' ? '空间连接' : '个人连接'} · {row.id}</strong><span>{row.state} · {row.kind === 'space' ? '空间账号' : row.owner_id ?? '账号未知'}</span>{canManage && row.state === 'active' ? <span><Button type="button" disabled={busy !== ''} onClick={() => void authorize(row)}>重新授权</Button> <Button type="button" disabled={busy !== ''} onClick={() => void revoke(row)}>撤销</Button></span> : null}{row.state === 'revoked' ? <small>本地连接已撤销，远端清理异步进行。</small> : null}</li>)}</ul>}</Card></main>;
}
