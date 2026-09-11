import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { RefundView } from '@weknora/contracts';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';
import { refundMessage } from './refund-state.ts';

function formatCny(amountFen: string): string {
  return `¥${(Number.parseInt(amountFen, 10) / 100).toFixed(2)}`;
}

type RefundLoadState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; refund: RefundView };

interface RefundPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
  /** The ONLY refund this page may query; set by the host and never replaced here. */
  refundId: string;
  /**
   * Original order label for display. RefundView intentionally carries only the
   * four contract fields (id/state/amount_fen/locked_credits), so the host
   * supplies the originating order id it already knows.
   */
  orderId?: string;
}

export function RefundPage({ client, scopeController, refundId, orderId }: RefundPageProps) {
  const [state, setState] = useState<RefundLoadState>({ status: 'loading' });
  const [reloadToken, setReloadToken] = useState(0);
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'commercial-refund'), [scope.scope]);

  // Re-query the SAME refund id: every retry (including a revocation re-drive)
  // reuses the original key so the backend reconciles instead of paying twice.
  useEffect(() => {
    let active = true;
    const currentScope = scopeController.current();
    setState({ status: 'loading' });
    void client.commercial.getRefund(refundId, currentScope.signal).then((refund) => {
      if (active && scopeController.isCurrent(currentScope.scope)) setState({ status: 'ready', refund });
    }).catch((error: unknown) => {
      if (!active || currentScope.signal.aborted || !scopeController.isCurrent(currentScope.scope)) return;
      setState({ status: 'error', message: error instanceof Error ? error.message : 'Unable to load refund' });
    });
    return () => { active = false; };
  }, [client, scopeController, refundId, reloadToken]);

  const refund = state.status === 'ready' ? state.refund : null;
  const notYetTerminal = refund !== null
    && refund.state !== 'completed'
    && refund.state !== 'failed_confirmed'
    && refund.state !== 'not_created_confirmed';

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Commercial</p>
          <h1>退款进度</h1>
          <p className="wk-muted">Scoped refund status from GET /api/v1/commercial/refunds/:id</p>
        </div>
        <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>重新核对（同一退款单）</Button>
      </header>
      <Card>
        <p className="wk-debug">scope key: {JSON.stringify(queryKey)}</p>
        {state.status === 'loading' ? <Status>Loading refund…</Status> : null}
        {state.status === 'error' ? (
          <>
            <Status tone="error">{state.message}</Status>
            <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>重试（复用原退款单号，不新建退款）</Button>
          </>
        ) : null}
        {state.status === 'ready' ? (
          <>
            <section aria-live="polite">
              <p>{refundMessage(state.refund.state)}</p>
              {notYetTerminal ? (
                <p className="wk-muted">
                  核对中：结果未到终态前，本页不提供对同一订单再次申请退款的新键；核对未知（unknown）时也仅重查原单。
                </p>
              ) : null}
            </section>
            <ul className="wk-list">
              <li><strong>退款单号</strong><span>{state.refund.id}</span></li>
              <li><strong>原订单</strong><span>{orderId ?? '（宿主未提供原订单号；本页仅按退款单号核对）'}</span></li>
              <li><strong>申请金额</strong><span>{formatCny(state.refund.amount_fen)}</span></li>
              <li><strong>锁定额度</strong><span>{state.refund.locked_credits}</span></li>
              <li><strong>核对状态</strong><span>{refundMessage(state.refund.state)}</span></li>
            </ul>
            {state.refund.state === 'revocation_pending' ? (
              <section>
                <p className="wk-muted">
                  渠道退款已成功、权益撤回仍在处理。下面的重试只针对原退款单 {state.refund.id} 重查/重驱权益调整，不产生新的退款业务操作。
                </p>
                <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>重试权益调整（原退款单 {state.refund.id}）</Button>
              </section>
            ) : null}
          </>
        ) : null}
      </Card>
    </main>
  );
}
