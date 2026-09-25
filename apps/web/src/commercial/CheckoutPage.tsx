import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ApiError } from '@weknora/api-client';
import type { WeKnoraClient } from '@weknora/api-client';
import type { OrderView, PurchaseView, QuoteView } from '@weknora/contracts';
import { isSafeCheckoutUrl } from '@weknora/contracts';
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

// 行项目 kind 的显示名（闭合集合；第一切片只有订阅费）。
function lineItemLabel(kind: string): string {
  return kind === 'subscription_fee' ? '订阅费' : kind;
}

// 冻结权益 key 的显示名（未知 key 原样显示——spec L210：页面词汇稳定，不含平台词汇）。
const FEATURE_LABELS: Record<string, string> = {
  advanced_models: '高级模型',
  api_access: 'API 访问',
  priority_support: '优先支持',
};

function isTerminal(order: OrderView): boolean {
  return order.fulfillment === 'fulfilled' || order.payment === 'closed';
}

// R1-V01：purchase 缺单/不可确认时的闭合失败文案（reason 是后端闭合令牌，
// state 是闭合产品状态；页面词汇稳定，不含平台词汇——spec L210）。
export function purchaseErrorMessage(purchase: PurchaseView): string {
  const closed = purchaseReasonMessage(purchase.reason);
  if (closed !== undefined) return closed;
  return purchase.state === 'canceled'
    ? '该购买已取消，请重新发起购买'
    : '购买未能创建，请稍后重试';
}

// 闭合 reason 令牌 → 中文文案。R1-24：POST /purchases 平台故障走 503 信封
// （{"error":..., "reason":"unreachable|..."}），ApiError.details.reason 携带
// 同一枚令牌——两条路径（GET 2xx 视图与 POST 503 错误）共用这一份映射。
export function purchaseReasonMessage(reason: unknown): string | undefined {
  switch (reason) {
    case 'unconfigured': return '支付渠道未配置，购买暂不可用';
    case 'unreachable': return '支付平台暂时不可达，请稍后重试';
    case 'invalid_response': return '支付平台返回异常，请稍后重试';
    case 'unsupported': return '当前环境暂不支持购买';
    default: return undefined;
  }
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
        // #81：提交走 payment-gated purchase。重试语义：同一 quote 重试就是同一次
        // purchase 调用，后端按身份幂等返回同一订单（不产生第二张订单/第二张账单）。
        const purchase = await client.commercial.purchase(
          { quote_id: quoteRef.current.id, provider: 'wechat' },
          currentScope.signal,
        );
        // R1-V01：purchase.order 缺席时不再发起空 ID 的 getOrder 请求（那必然
        // 产生 /orders/ 的 404/路由错配，且丢失闭合 reason）。按闭合
        // state/reason 渲染失败文案；正常路径后端必带 order。
        if (!purchase.order) {
          throw new Error(purchaseErrorMessage(purchase));
        }
        // From here on this page only re-queries the same order id; it never creates another order.
        const order = purchase.order;
        orderIdRef.current = order.id;
        if (active && scopeController.isCurrent(currentScope.scope)) {
          setState({ status: 'ready', order, quote: quoteRef.current });
        }
      } catch (error) {
        if (!active || !scopeController.isCurrent(currentScope.scope)) return;
        // R1-24：POST /purchases 的平台故障以 503 + 顶层闭合 reason 令牌回答，
        // ApiError.details.reason 携带它——优先映射闭合文案，而不是把英文
        // 原始 message（"purchase temporarily unavailable"）直接展示给用户。
        const closedReason = error instanceof ApiError
          ? purchaseReasonMessage((error.details as { reason?: unknown } | undefined)?.reason)
          : undefined;
        setState({
          status: 'error',
          message: closedReason ?? (error instanceof Error ? error.message : 'Unable to open checkout'),
        });
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
              {/* AC3：购买成功后产品状态为「待付款」，付费权益未开通——仅当订单
                  仍在待付款时显示（R1-V12：已支付/终态不再误导重复支付）。
                  R1-V19：Status 本身渲染 <p>，不再嵌套外层段落。 */}
              {state.order.payment === 'pending' ? <Status>待付款（权益未开通）</Status> : null}
              {/* 渠道支付入口（审查 F2）：渠道请求创建后展示跳转链接，用户由此完成支付。
                  R1-V13：渲染前经 scheme 白名单校验，危险 scheme 一律不渲染。 */}
              {state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url) ? (
                <p>
                  <a href={state.order.checkout_url} target="_blank" rel="noreferrer">前往支付</a>
                </p>
              ) : null}
              <Button type="button" onClick={refreshOrder}>刷新订单状态</Button>
            </section>
            {/* AC1：报价冻结面——币种、行项目、权益与过期时间原样呈现。 */}
            {state.quote ? (
              <section>
                <h3>报价明细（{state.quote.currency ?? 'CNY'}）</h3>
                {state.quote.line_items?.length ? (
                  <ul className="wk-list">
                    {state.quote.line_items.map((li, i) => (
                      <li key={`${li.kind}-${i}`}>
                        <strong>{lineItemLabel(li.kind)}{li.name ? `（${li.name}）` : ''}</strong>
                        <span>{formatCny(li.amount_fen)}</span>
                      </li>
                    ))}
                  </ul>
                ) : null}
                {state.quote.features ? (
                  <p className="wk-muted">
                    包含权益：
                    {Object.entries(state.quote.features).filter(([, on]) => on).map(([key]) => FEATURE_LABELS[key] ?? key).join('、')}
                  </p>
                ) : null}
                <p className="wk-muted">报价有效期至：{new Date(state.quote.expires_at).toLocaleString()}</p>
              </section>
            ) : null}
            <ul className="wk-list">
              <li><strong>订单号</strong><span>{state.order.id}</span></li>
              <li><strong>人民币实付</strong><span>{formatCny(state.order.amount_fen)}</span></li>
              <li><strong>计费周期</strong><span>月周期（按月结算）</span></li>
              <li><strong>套餐金额</strong><span>{state.quote ? `${formatCny(state.quote.amount_fen)}（按报价折算）` : '按实际报价折算'}</span></li>
            </ul>
            <p className="wk-muted">{RECHARGE_EXPIRY_NOTE}</p>
          </>
        ) : null}
      </Card>
    </main>
  );
}
