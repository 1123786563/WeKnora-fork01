import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { CommercialSummary, CommercialUsageRow } from '@weknora/contracts';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';

export type CommercialSummaryState =
  | { status: 'success'; summary: CommercialSummary }
  | { status: 'error'; message: string };

export type CommercialUsageState =
  | { status: 'success'; rows: CommercialUsageRow[] }
  | { status: 'error'; message: string };

export async function loadCommercialSummary(
  client: Pick<WeKnoraClient['commercial'], 'summary'>,
  signal?: AbortSignal,
): Promise<CommercialSummaryState> {
  try {
    return { status: 'success', summary: await client.summary(signal) };
  } catch (error) {
    if (signal?.aborted) throw error;
    return { status: 'error', message: error instanceof Error ? error.message : 'Unable to load billing summary' };
  }
}

export async function loadCommercialUsage(
  client: Pick<WeKnoraClient['commercial'], 'usage'>,
  signal?: AbortSignal,
): Promise<CommercialUsageState> {
  try {
    return { status: 'success', rows: await client.usage(signal) };
  } catch (error) {
    if (signal?.aborted) throw error;
    return { status: 'error', message: error instanceof Error ? error.message : 'Unable to load resource usage' };
  }
}

/** 套餐行的显示名：已购空间显示 plan_key，base_tier 空间回退 base_tier_key 或「基础版」。 */
export function planDisplayName(summary: CommercialSummary): string {
  return summary.base_tier || !summary.subscription
    ? (summary.base_tier_key ?? '基础版')
    : summary.subscription.plan_key;
}

interface BillingPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
}

// 与 UsagePanel 模型表格同款样式（tailwind utilities）。
const USAGE_TABLE = 'w-full border-collapse text-[13px]';
const USAGE_TABLE_CELL = 'border-b border-[#eef1f5] px-[10px] py-[8px] text-left';
const USAGE_NUMBER_CELL = USAGE_TABLE_CELL + ' text-right tabular-nums';

export function BillingPage({ client, scopeController }: BillingPageProps) {
  const [reloadToken, setReloadToken] = useState(0);
  const [state, setState] = useState<CommercialSummaryState>({ status: 'error', message: 'Loading…' });
  const [usageState, setUsageState] = useState<CommercialUsageState>({ status: 'error', message: 'Loading…' });
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'commercial-summary'), [scope.scope]);
  // Space label comes from the current scope tenant; fall back to a neutral label
  // until the shell provides a tenant display name.
  const spaceName = scope.scope.tenantId ? `空间 ${scope.scope.tenantId}` : '当前空间';

  useEffect(() => {
    let active = true;
    setState({ status: 'error', message: 'Loading…' });
    setUsageState({ status: 'error', message: 'Loading…' });
    void loadCommercialSummary(client.commercial, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setState(next);
    });
    void loadCommercialUsage(client.commercial, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setUsageState(next);
    });
    return () => { active = false; };
  }, [client, reloadToken, scopeController, scope.scope, scope.signal]);

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Commercial</p>
          <h1>{spaceName} · 账单与套餐</h1>
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
          <ul className="wk-list" data-testid="billing-summary-list">
            <li><strong>套餐</strong><span>{planDisplayName(state.summary)}</span></li>
            <li><strong>到期</strong><span>{state.summary.subscription?.paid_until || '无固定到期（未订阅）'}</span></li>
          </ul>
        ) : null}
      </Card>
      {usageState.status === 'success' ? (
        <Card>
          <h2>资源用量</h2>
          {usageState.rows.length === 0 ? (
            <Status>暂无资源用量数据。</Status>
          ) : (
            <table className={USAGE_TABLE} data-testid="billing-usage-table">
              <thead>
                <tr className="border-b border-[#e7e7ea]">
                  <th className={USAGE_TABLE_CELL + ' font-semibold'}>资源</th>
                  <th className={USAGE_NUMBER_CELL + ' font-semibold'}>已用</th>
                  <th className={USAGE_NUMBER_CELL + ' font-semibold'}>上限</th>
                </tr>
              </thead>
              <tbody>
                {usageState.rows.map((row) => (
                  <tr key={row.resource}>
                    <td className={USAGE_TABLE_CELL}>{row.resource}</td>
                    <td className={USAGE_NUMBER_CELL}>{row.used}</td>
                    <td className={USAGE_NUMBER_CELL}>{row.limit === null ? '∞' : row.limit}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>
      ) : usageState.message !== 'Loading…' ? (
        <Card>
          <Status tone="error">{usageState.message}</Status>
        </Card>
      ) : null}
    </main>
  );
}
