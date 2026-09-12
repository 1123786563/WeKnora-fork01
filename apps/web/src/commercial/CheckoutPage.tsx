import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { OrderView, QuoteView } from '@weknora/contracts';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';
import { orderMessage } from './order-state.ts';

const POLL_INTERVAL_MS = 3000;
const DEFAULT_PLAN_KEY = 'pro';
const DEFAULT_PLAN_VERSION = 1;
const DEFAULT_SUBSCRIPTION_VERSION = 1;
const RECHARGE_EXPIRY_NOTE = '充值12个月到期说明：充值额度自到账之日起 12 个月内有效，到期未使用的额度将失效，不自动续期。';

type CheckoutState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; order: OrderView; quote: QuoteView | null };

function formatCny(amountFen: string): string {
  return `¥${(Number.parseInt(amountFen, 10) / 100).toFixed(2)}`;
}

function isTerminal(order: OrderView): boolean {
  return order.fulfillment === 'fulfilled' || order.payment === 'closed';
}

interface CheckoutPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
  orderId: string;
}

export function CheckoutPage({ client, scopeController, orderId }: CheckoutPageProps) {
  const [state, setState] = useState<CheckoutState>({ status: 'loading' });
  const [retryToken, setRetryToken] = useState(0);
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'commercial-checkout'), [scope.scope]);
  // The only order this page may ever query or create; set once, never replaced by a new purchase.
  const orderIdRef = useRef<string>(orderId);
  // Generated at most once per mount; every retry (including a lost createOrder response)
  // reuses this same key so the backend deduplicates instead of charging twice.
  const idempotencyKeyRef = useRef<string>('');
  const quoteRef = useRef<QuoteView | null>(null);

  useEffect(() => {
    let active = true;
    const currentScope = scopeController.current();
    setState({ status: 'loading' });

    async function run(): Promise<void> {
      try {
        if (orderIdRef.current) {
          const order = await client.commercial.getOrder(orderIdRef.current, currentScope.signal);
          if (active && scopeController.isCurrent(currentScope.scope)) {
            setState({ status: 'ready', order, quote: quoteRef.current });
          }
          return;
        }
        if (!quoteRef.current) {
          quoteRef.current = await client.commercial.quote(
            { plan_key: DEFAULT_PLAN_KEY, plan_version: DEFAULT_PLAN_VERSION, subscription_version: DEFAULT_SUBSCRIPTION_VERSION },
            currentScope.signal,
          );
          if (!active || !scopeController.isCurrent(currentScope.scope)) return;
        }
        if (!idempotencyKeyRef.current) {
          idempotencyKeyRef.current = `checkout-${scope.scope.tenantId ?? 'none'}-${crypto.randomUUID()}`;
        }
        const order = await client.commercial.createOrder(
          { quote_id: quoteRef.current.id, provider: 'wechat', idempotency_key: idempotencyKeyRef.current },
          currentScope.signal,
        );
        // From here on this page only re-queries the same order id; it never creates another order.
        orderIdRef.current = order.id;
        if (active && scopeController.isCurrent(currentScope.scope)) {
          setState({ status: 'ready', order, quote: quoteRef.current });
        }
      } catch (error) {
        if (!active || !scopeController.isCurrent(currentScope.scope)) return;
        setState({ status: 'error', message: error instanceof Error ? error.message : 'Unable to open checkout' });
      }
    }

    void run();
    // Unmount/scope switch: active=false drops late results; in-flight requests are
    // aborted by the scope signal. Background fulfillment is never cancelled by leaving.
    return () => { active = false; };
  }, [client, scopeController, retryToken, scope.scope.tenantId]);

  const refreshOrder = useCallback((): void => {
    const id = orderIdRef.current;
    if (!id) return;
    const currentScope = scopeController.current();
    void client.commercial.getOrder(id, currentScope.signal).then((order) => {
      if (scopeController.isCurrent(currentScope.scope)) {
        setState((prev) => (prev.status === 'ready' ? { ...prev, order } : prev));
      }
    }).catch((error: unknown) => {
      if (currentScope.signal.aborted || !scopeController.isCurrent(currentScope.scope)) return;
      setState((prev) => (prev.status === 'ready'
        ? prev
        : { status: 'error', message: error instanceof Error ? error.message : 'Unable to load order' }));
    });
  }, [client, scopeController]);

  const order = state.status === 'ready' ? state.order : null;

  // Poll the SAME order id after payment until a terminal state; each tick is guarded by
  // isCurrent and the interval is cleared on unmount or scope switch.
  useEffect(() => {
    if (!order || isTerminal(order)) return undefined;
    const timer = window.setInterval(() => { refreshOrder(); }, POLL_INTERVAL_MS);
    return () => { window.clearInterval(timer); };
  }, [refreshOrder, order?.id, order?.payment, order?.fulfillment]);

  const spaceName = scope.scope.tenantId ? `空间 ${scope.scope.tenantId}` : '当前空间';

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Commercial</p>
          <h1>订单结算</h1>
          <p className="wk-muted">Scoped order status from GET /api/v1/commercial/orders/:id</p>
        </div>
      </header>
      <Card>
        <p className="wk-debug">scope key: {JSON.stringify(queryKey)}</p>
        {state.status === 'loading' ? <Status>Loading checkout…</Status> : null}
        {state.status === 'error' ? (
          <>
            <Status tone="error">{state.message}</Status>
            <Button type="button" onClick={() => setRetryToken((value) => value + 1)}>重试（复用原订单与幂等键，不重复下单）</Button>
          </>
        ) : null}
        {state.status === 'ready' ? (
          <>
            <section aria-live="polite">
              <h2>{spaceName} 的订单</h2>
              <p>{orderMessage(state.order)}</p>
              <Button type="button" onClick={refreshOrder}>刷新订单状态</Button>
            </section>
            <ul className="wk-list">
              <li><strong>订单号</strong><span>{state.order.id}</span></li>
              <li><strong>人民币实付</strong><span>{formatCny(state.order.amount_fen)}</span></li>
              <li><strong>计费周期</strong><span>月周期（按月结算）</span></li>
              <li><strong>升级差价</strong><span>{state.quote ? `${formatCny(state.quote.amount_fen)}（按报价折算，已含升级差价）` : '按实际报价折算'}</span></li>
            </ul>
            <p className="wk-muted">{RECHARGE_EXPIRY_NOTE}</p>
          </>
        ) : null}
      </Card>
    </main>
  );
}
