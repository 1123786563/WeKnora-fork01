import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain/scope';
import { Button as TButton } from 'tdesign-react';
import { WkCard as Card, WkStatus as Status } from '../shared/wk-legacy.tsx';
import { appsApi, type CatalogEntry } from './api.ts';
import { catalogPublishedLabel, catalogRiskLabel, catalogRiskTone, installationStateLabel } from './connection-state.ts';

interface AppsPageProps { client: WeKnoraClient; scopeController: ReturnType<typeof createScopeController> }

export function AppsPage({ client, scopeController }: AppsPageProps) {
  const [catalog, setCatalog] = useState<CatalogEntry[]>([]); const [installations, setInstallations] = useState<Awaited<ReturnType<ReturnType<typeof appsApi>['listInstallations']>>>([]); const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading'); const [error, setError] = useState(''); const [reload, setReload] = useState(0);
  useEffect(() => { const scope = scopeController.current(); const api = appsApi(client); setState('loading'); setError(''); setCatalog([]); setInstallations([]); void Promise.all([api.listCatalog(scope.signal), api.listInstallations(scope.signal)]).then(([nextCatalog, nextInstallations]) => { if (scopeController.isCurrent(scope.scope)) { setCatalog(nextCatalog); setInstallations(nextInstallations); setState('ready'); } }).catch((e) => { if (!scope.signal.aborted && scopeController.isCurrent(scope.scope)) { setError(e instanceof Error ? e.message : 'Unable to load app catalog'); setState('error'); } }); }, [client, reload, scopeController]);
  return <main className="wk-page"><header className="wk-header"><div><p className="wk-eyebrow">Apps</p><h1>应用目录</h1><p className="wk-muted">已授权空间可见的已发布操作与安装版本</p></div><TButton type="button" onClick={() => setReload((value) => value + 1)} disabled={state === 'loading'}>刷新</TButton></header><Card>{state === 'loading' ? <Status>正在加载应用目录…</Status> : null}{state === 'error' ? <><Status tone="error">{error}</Status><TButton type="button" onClick={() => setReload((value) => value + 1)}>重试</TButton></> : null}{state === 'ready' ? <><h2>可用操作</h2>{catalog.length === 0 ? <Status>暂无可用操作。</Status> : <ul className="wk-list">{catalog.map((row) => <li key={row.action_id}><strong>{row.action_id}</strong><span>{row.app_id} @ {row.app_version} · {row.provider} · 风险 <Status tone={catalogRiskTone(row.risk)}>{catalogRiskLabel(row.risk)}</Status> · 权限 {row.required_scopes.join('、') || '无'} · {catalogPublishedLabel(row.published)}</span><small>schema {row.schema_digest}</small></li>)}</ul>}<h2>已安装版本</h2>{installations.length === 0 ? <Status>暂无安装版本。</Status> : <ul className="wk-list">{installations.map((row) => <li key={row.id}><strong>{row.app_key} @ {row.version}</strong><span>{installationStateLabel(row.state)} · 权限 {row.scopes.join('、') || '无'}</span></li>)}</ul>}</> : null}</Card></main>;
}
