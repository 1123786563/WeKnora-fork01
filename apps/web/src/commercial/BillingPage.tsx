import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { CommercialSummary } from '@weknora/contracts';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';

export type CommercialSummaryState =
  | { status: 'success'; summary: CommercialSummary }
  | { status: 'error'; message: string };

export async function loadCommercialSummary(
  client: Pick<WeKnoraClient, 'commercial'>,
  signal?: AbortSignal,
): Promise<CommercialSummaryState> {
  try {
    return { status: 'success', summary: await client.commercial.summary(signal) };
  } catch (error) {
    if (signal?.aborted) throw error;
    return { status: 'error', message: error instanceof Error ? error.message : 'Unable to load billing summary' };
  }
}

interface BillingPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
}

export function BillingPage({ client, scopeController }: BillingPageProps) {
  const [reloadToken, setReloadToken] = useState(0);
  const [state, setState] = useState<CommercialSummaryState>({ status: 'error', message: 'Loading…' });
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'commercial-summary'), [scope.scope]);
  // Space label comes from the current scope tenant; fall back to a neutral label
  // until the shell provides a tenant display name.
  const spaceName = scope.scope.tenantId ? `空间 ${scope.scope.tenantId}` : '当前空间';

  useEffect(() => {
    let active = true;
    setState({ status: 'error', message: 'Loading…' });
    void loadCommercialSummary(client, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setState(next);
    });
    return () => { active = false; };
  }, [client, reloadToken, scopeController, scope.scope, scope.signal]);

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Commercial</p>
          <h1>{spaceName} · 账单与额度</h1>
          <p className="wk-muted">Live data from GET /api/v1/commercial/summary</p>
        </div>
        <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Reload</Button>
      </header>
      <Card>
        <p className="wk-debug">scope key: {JSON.stringify(queryKey)}</p>
        {state.status === 'error' && state.message === 'Loading…' ? <Status>Loading billing summary…</Status> : null}
        {state.status === 'error' && state.message !== 'Loading…' ? (
          <>
            <Status tone="error">{state.message}</Status>
            <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Try again</Button>
          </>
        ) : null}
        {state.status === 'success' ? (
          <ul className="wk-list">
            <li><strong>套餐</strong><span>{state.summary.plan_name}</span></li>
            <li><strong>到期</strong><span>{state.summary.paid_until ?? '无固定到期（未订阅）'}</span></li>
            <li><strong>可用额度</strong><span>{state.summary.available}</span></li>
            <li><strong>占用额度</strong><span>{state.summary.held}</span></li>
            <li><strong>退款锁定</strong><span>{state.summary.refund_locked}</span></li>
            <li><strong>数据截至</strong><span>{state.summary.as_of}</span></li>
          </ul>
        ) : null}
        {state.status === 'success' && state.summary.stale ? (
          <Status tone="error">数据可能滞后（stale），以对账后的余额为准。</Status>
        ) : null}
      </Card>
    </main>
  );
}
