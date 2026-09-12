import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { ConnectionView, InstallationView, SyncStatusView } from '@weknora/contracts';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';
import { connectionLabel, pauseReasonLabel } from './connection-state.ts';

export type AppsPageState =
  | { status: 'success'; installations: InstallationView[]; connections: ConnectionView[] }
  | { status: 'error'; message: string };

export async function loadAppsOverview(
  client: Pick<WeKnoraClient, 'apps'>,
  signal?: AbortSignal,
): Promise<AppsPageState> {
  try {
    const [installations, connections] = await Promise.all([
      client.apps.listInstallations(signal),
      client.apps.listConnections(signal),
    ]);
    return { status: 'success', installations, connections };
  } catch (error) {
    if (signal?.aborted) throw error;
    return { status: 'error', message: error instanceof Error ? error.message : 'Unable to load apps' };
  }
}

interface AppsPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
  /** Optional tenant role; only owner/admin see install/upgrade actions — the server remains authoritative. */
  role?: string;
  /** Data-source ids whose sync status should be surfaced; when omitted the sync section stays empty. */
  syncDatasourceIds?: string[];
}

export function AppsPage({ client, scopeController, role, syncDatasourceIds = [] }: AppsPageProps) {
  const [reloadToken, setReloadToken] = useState(0);
  const [state, setState] = useState<AppsPageState>({ status: 'error', message: 'Loading…' });
  const [syncStatuses, setSyncStatuses] = useState<Record<string, SyncStatusView | string>>({});
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'apps-overview'), [scope.scope]);
  const canInstall = role === 'owner' || role === 'admin';

  useEffect(() => {
    let active = true;
    setState({ status: 'error', message: 'Loading…' });
    void loadAppsOverview(client, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setState(next);
    });
    return () => { active = false; };
  }, [client, reloadToken, scopeController, scope.scope, scope.signal]);

  // Sync status loads are scope-guarded per data source: a late response from
  // a previous space switch is dropped by isCurrent and never overwrites the view.
  useEffect(() => {
    let active = true;
    if (syncDatasourceIds.length === 0) { setSyncStatuses({}); return; }
    for (const dsId of syncDatasourceIds) {
      void client.apps.getSyncStatus(dsId, scope.signal).then(
        (status) => { if (active && scopeController.isCurrent(scope.scope)) setSyncStatuses((prev) => ({ ...prev, [dsId]: status })); },
        (error) => {
          if (scope.signal.aborted) return;
          if (active && scopeController.isCurrent(scope.scope)) {
            setSyncStatuses((prev) => ({ ...prev, [dsId]: error instanceof Error ? error.message : '同步状态获取失败' }));
          }
        },
      );
    }
    return () => { active = false; };
  }, [client, reloadToken, scopeController, scope.scope, scope.signal, syncDatasourceIds]);

  const installations = state.status === 'success' ? state.installations : [];
  const connections = state.status === 'success' ? state.connections : [];

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Apps</p>
          <h1>应用目录与连接</h1>
          <p className="wk-muted">Live data from /api/v1/apps/installations and /apps/connections</p>
        </div>
        <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Reload</Button>
      </header>

      <Card>
        <p className="wk-debug">scope key: {JSON.stringify(queryKey)}</p>
        {state.status === 'error' && state.message === 'Loading…' ? <Status>Loading apps…</Status> : null}
        {state.status === 'error' && state.message !== 'Loading…' ? (
          <>
            <Status tone="error">{state.message}</Status>
            <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Try again</Button>
          </>
        ) : null}

        {state.status === 'success' ? (
          <>
            <h2>安装版本与费用/权限说明</h2>
            {installations.length === 0 ? <Status>尚未安装任何应用。</Status> : (
              <ul className="wk-list">
                {installations.map((installation) => (
                  <li key={installation.id}>
                    <strong>{installation.app_key} @ {installation.version}</strong>
                    <span>{installation.scopes.length > 0 ? '权限范围：' + installation.scopes.join('、') : '无额外权限'}{' · '}按量计费，详见账单</span>
                  </li>
                ))}
              </ul>
            )}
            {!canInstall ? (
              <Status>普通成员不能直接安装或升级应用。如需新应用或新版本，请通过申请入口向空间管理员提交申请。</Status>
            ) : null}

            <h2>个人/空间连接</h2>
            {connections.length === 0 ? <Status>暂无连接。创建连接需完成应用方授权。</Status> : (
              <ul className="wk-list">
                {connections.map((connection) => (
                  <li key={connection.id}>
                    <strong>{connectionLabel(connection)}</strong>
                    <span>{connection.id}</span>
                    {connection.state === 'pending_reauthorization' ? (
                      <Status tone="error">连接等待重新授权，恢复前不会执行同步。</Status>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}

            <h2>资源范围</h2>
            {installations.length === 0 ? <Status>未安装应用时没有资源范围。</Status> : (
              <ul className="wk-list">
                {installations.map((installation) => (
                  <li key={'scope-' + installation.id}>
                    <strong>{installation.app_key}</strong>
                    <span>{installation.scopes.length > 0 ? installation.scopes.join('、') : '（无）'}</span>
                  </li>
                ))}
              </ul>
            )}

            <h2>目标知识库</h2>
            <Status>同步内容写入的目标知识库由各数据源配置决定；此处跟随数据源设置，不在应用页修改。</Status>

            <h2>同步进度与暂停原因</h2>
            {syncDatasourceIds.length === 0 ? (
              <Status>未提供数据源，无法展示同步状态。</Status>
            ) : (
              <ul className="wk-list">
                {syncDatasourceIds.map((dsId) => {
                  const status = syncStatuses[dsId];
                  if (status === undefined) return <li key={dsId}><strong>{dsId}</strong><span>加载中…</span></li>;
                  if (typeof status === 'string') return <li key={dsId}><strong>{dsId}</strong><span>{status}</span></li>;
                  const pause = pauseReasonLabel(status.pause_reason);
                  const stateText = status.state + (pause !== null ? '（' + pause + '）' : '') + (status.requires_reauthorization ? ' · 需重新授权' : '');
                  return (
                    <li key={dsId}>
                      <strong>{dsId}</strong>
                      <span>{stateText}</span>
                      {status.binding !== null ? <span>经由连接 {status.binding.connection_id}（auth v{status.binding.auth_version}）</span> : null}
                    </li>
                  );
                })}
              </ul>
            )}
          </>
        ) : null}
      </Card>
    </main>
  );
}