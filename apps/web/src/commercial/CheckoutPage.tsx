import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ApiError } from '@weknora/api-client';
import type { WeKnoraClient } from '@weknora/api-client';
import type { OrderView, PurchaseView, QuoteView } from '@weknora/contracts';
import { isSafeCheckoutUrl } from '@weknora/contracts';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button } from 'tdesign-react';
import { Card, Status } from './ui.tsx';
import { orderMessage, PURCHASE_STATE_LABEL } from './order-state.ts';

const POLL_INTERVAL_MS = 3000;
const DEFAULT_PLAN_KEY = 'pro';
const DEFAULT_PLAN_VERSION = 1;
const DEFAULT_SUBSCRIPTION_VERSION = 1;
const RECHARGE_EXPIRY_NOTE = '充值12个月到期说明：充值额度自到账之日起 12 个月内有效，到期未使用的额度将失效，不自动续期。';

type PaymentChannel = 'alipay' | 'wechat';

type CheckoutState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; order: OrderView; quote: QuoteView | null; purchase?: PurchaseView };

// (#82 D3/审查 H1) The three product states the checkout surfaces:
// awaiting payment / paid awaiting activation / active — driven by the
// purchase projection (client.commercial.purchaseStatus), with the order's
// channel face as the fallback. (OCR84-R1-15 / D15-f) The purchase-driven
// branch consumes the ONE shared vocabulary PURCHASE_STATE_LABEL
// (order-state.ts) — this page no longer keeps a private copy of the words
// that had drifted from the Billing row's («已付款待激活» / «已生效» /
// «待付款（权益未开放）»). The order channel-face FALLBACK branches keep
// their own order-side wording unchanged (they describe the channel face,
// not the purchase state).
export function purchaseStateMessage(purchase: PurchaseView | undefined, order: OrderView): string {
  if (purchase) {
    const label = PURCHASE_STATE_LABEL[purchase.state];
    if (label) return label;
  }
  // (D15-f) 显式 if 链——三层嵌套三元不再积累。
  if (order.payment === 'pending') return '待付款（权益未开通）';
  if (order.payment === 'paid') return '已付款，权益处理中';
  if (order.fulfillment === 'fulfilled') return '权益已生效';
  return orderMessage(order);
}

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

// R2-01：POST /purchases 的 409/500 错误族不带 reason 令牌，只有 message 上的
// 闭合机器令牌（"quote expired"、"invoice_quote_mismatch"、…）——按 message
// 令牌映射中文文案；服务端错误（4xx/5xx ApiError）未命中已知令牌时给统一
// 中文兜底，绝不把英文机器令牌原文展示给用户（spec L210：页面词汇稳定，
// 不含平台词汇）。
const PURCHASE_CONFLICT_MESSAGES: Record<string, string> = {
  'quote expired': '报价已过期，请重新获取报价',
  'invoice_quote_mismatch': '订单金额与报价不一致，请重新发起购买',
  'purchase_plan_conflict': '购买套餐已发生变更，请重新发起购买',
  'purchase_not_awaiting_payment': '当前购买状态不支持重复支付',
  'quote already used': '报价已被使用，请重新获取报价',
  'subscription changed since the quote was cut; please re-quote': '订阅已变更，请重新获取报价',
  'quote predates the purchase freeze; please re-quote': '报价已过期，请重新获取报价',
  'this plan version is not purchasable yet; please re-quote later': '该套餐版本暂不可购，请稍后重试',
  'purchase failed': '购买未能创建，请稍后重试',
};
const PURCHASE_ERROR_FALLBACK = '购买未能创建，请稍后重试';

// R3-07：报价级冲突令牌——同一 quote 的重试是死胡同（后端按 quote 查库
// 确定性返回同一 409），重试必须走一次新报价（catch 中清空 quoteRef）。
// (D15-e) invoice_quote_mismatch 与 not-purchasable-yet 同属报价级死胡同：
// 两者的成因都冻结在 quote 侧（权威面金额不一致 / 版本不可购），同一 quote
// 重试必然拿到同一 409——不补则重试死循环。
const QUOTE_LEVEL_CONFLICT_TOKENS = new Set<string>([
  'quote expired',
  'quote already used',
  'quote predates the purchase freeze; please re-quote',
  'subscription changed since the quote was cut; please re-quote',
  'invoice_quote_mismatch',
  'this plan version is not purchasable yet; please re-quote later',
]);

export function isQuoteLevelConflict(error: unknown): boolean {
  return error instanceof ApiError && QUOTE_LEVEL_CONFLICT_TOKENS.has(error.message);
}

export function purchaseConflictMessage(message: string | undefined): string | undefined {
  if (message === undefined) return undefined;
  return PURCHASE_CONFLICT_MESSAGES[message];
}

// error→展示文案（R2-01/R3-02）：reason 令牌优先（503 信封），其次 message
// 令牌（409/500 族）；服务端 ApiError 未命中任何已知令牌时给统一中文兜底；
// 非服务端错误（网络层等）同样收敛为中文兜底——原始 message 仅进 console，
// 英文技术串不渲染给用户（spec L210）。
export function purchaseErrorText(error: unknown): string {
  if (error instanceof ApiError) {
    const closedReason = purchaseReasonMessage((error.details as { reason?: unknown } | undefined)?.reason);
    if (closedReason !== undefined) return closedReason;
    const conflict = purchaseConflictMessage(error.message);
    if (conflict !== undefined) return conflict;
    if (error.status !== undefined && error.status >= 400) return PURCHASE_ERROR_FALLBACK;
  }
  if (error instanceof Error) console.warn('checkout purchase failed:', error.message);
  return '网络异常，请稍后重试';
}

interface CheckoutPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
  orderId: string;
}

export function CheckoutPage({ client, scopeController, orderId }: CheckoutPageProps) {
  const [state, setState] = useState<CheckoutState>({ status: 'loading' });
  const [retryToken, setRetryToken] = useState(0);
  // (审查 H1) The payment-channel selector: Alipay is the #82 main rail and
  // therefore the default; the WeChat path stays selectable (#81 shape).
  const [channel, setChannel] = useState<PaymentChannel>('alipay');
  // (D15-b) The radio's latest value rides a ref: the loading effect does NOT
  // depend on the channel anymore (switching the radio must not re-run the
  // load / re-issue requests); the submit reads this ref.
  const channelRef = useRef<PaymentChannel>('alipay');
  // (D15-c) The channel the CURRENT order was submitted with — null until
  // this page has actually submitted a purchase (a deep-link open never
  // guesses the order's channel). Switching the radio on a still-pending
  // order offers an explicit re-submit entry (never a silent second order).
  const [submittedChannel, setSubmittedChannel] = useState<PaymentChannel | null>(null);
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'commercial-checkout'), [scope.scope]);
  // The only order this page may ever query or create; set once, cleared
  // ONLY by the explicit restart entry (R3-09), never by a new purchase.
  const orderIdRef = useRef<string | null>(orderId);
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
            void client.commercial.purchaseStatus(currentScope.signal).then((purchase) => {
              if (active && scopeController.isCurrent(currentScope.scope)) {
                setState((prev) => (prev.status === 'ready' ? { ...prev, purchase } : prev));
              }
            }).catch(() => { /* silent degrade */ });
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
        // (D15-b) 提交体读 channelRef——effect 不再依赖 channel。
        setSubmittedChannel(channelRef.current);
        const purchase = await client.commercial.purchase(
          { quote_id: quoteRef.current.id, provider: channelRef.current },
          currentScope.signal,
        );
        // R1-V01：purchase.order 缺席时不再发起空 ID 的 getOrder 请求（那必然
        // 产生 /orders/ 的 404/路由错配，且丢失闭合 reason）。按闭合
        // state/reason 渲染失败文案；正常路径后端必带 order。
        // (R3-02) 直接落错误态——不经过 throw/catch（catch 的统一兜底会把
        // 这条已映射的闭合文案再兜成「网络异常」）。
        // (D15-a) error setState 补 scope 守卫（与 ready setState 对齐）。
        if (!purchase.order) {
          if (active && scopeController.isCurrent(currentScope.scope)) {
            setState({ status: 'error', message: purchaseErrorMessage(purchase) });
          }
          return;
        }
        // From here on this page only re-queries the same order id; it never creates another order.
        // (D15-a) The order-id latch writes ONLY while this run still owns the
        // scope — a late resolution after unmount/scope switch must not pin a
        // stale id onto the next mount.
        const order = purchase.order;
        if (active && scopeController.isCurrent(currentScope.scope)) {
          orderIdRef.current = order.id;
          setState({ status: 'ready', order, quote: quoteRef.current, purchase });
        }
      } catch (error) {
        if (!active || !scopeController.isCurrent(currentScope.scope)) return;
        // R3-07：报价级冲突（过期/已消费/前置版本）对同一 quote 是确定性的
        // ——重试前清空缓存的报价，让「重试」走一次新报价而不是死循环。
        if (isQuoteLevelConflict(error)) {
          quoteRef.current = null;
        }
        // (D15-d) 文案分流：深链读单失败（orderIdRef 已持有 id，getOrder 抛
        // 错）是「订单加载失败」——不是「购买未能创建」（本页根本没发起购
        // 买）；购买创建失败继续走 purchaseErrorText 的闭合令牌映射。
        if (orderIdRef.current) {
          setState({ status: 'error', message: '订单加载失败，请稍后重试' });
          return;
        }
        // R1-V24/R2-01：POST /purchases 的失败面按闭合令牌映射（503 reason、
        // 409/500 message 令牌、统一中文兜底）——英文机器令牌/原始 message
        // 不直接展示给用户（spec L210）。
        setState({ status: 'error', message: purchaseErrorText(error) });
      }
    }

    void run();
    // Unmount/scope switch: active=false drops late results; in-flight requests are
    // aborted by the scope signal. Background fulfillment is never cancelled by leaving.
    // (D15-b) channel 左侧依赖已移除（channelRef 承载），切换单选不再重跑加载。
    return () => { active = false; };
  }, [client, scopeController, retryToken, scope.scope.tenantId]);

  const refreshOrder = useCallback((): void => {
    const id = orderIdRef.current;
    if (!id) return;
    const currentScope = scopeController.current();
    // (OCR r4 / 审查 M6) The three-state face rides the purchase projection;
    // the order read stays the channel fallback. A purchaseStatus failure
    // degrades silently — it must never block the order poll. Late
    // resolutions are dropped by the scope guard pair (isCurrent + abort).
    // (OCR84-R1-06) The scope pair alone does NOT cover restartCheckout: a
    // restart clears/repins orderIdRef WITHOUT aborting the scope, so an
    // in-flight getOrder issued for the ABANDONED order would still pass
    // live() and overwrite the new order back onto the abandoned pending
    // row (and its stale checkout link — a channel-switch user could pay
    // BOTH orders). The order identity guard closes it: a resolution is
    // live only while orderIdRef STILL pins the id it was issued for.
    const live = (): boolean =>
      !currentScope.signal.aborted &&
      scopeController.isCurrent(currentScope.scope) &&
      orderIdRef.current === id;
    void client.commercial.getOrder(id, currentScope.signal).then((order) => {
      if (live()) {
        setState((prev) => (prev.status === 'ready' ? { ...prev, order } : prev));
      }
      void client.commercial.purchaseStatus(currentScope.signal).then((purchase) => {
        if (live()) {
          setState((prev) => (prev.status === 'ready' ? { ...prev, purchase } : prev));
        }
      }).catch(() => { /* silent degrade */ });
    }).catch((error: unknown) => {
      if (!live()) return;
      // (D15-d) 轮询刷新失败收敛为闭合中文文案（spec L210：页面词汇稳定）。
      if (error instanceof Error) console.warn('checkout order refresh failed:', error.message);
      setState((prev) => (prev.status === 'ready'
        ? prev
        : { status: 'error', message: '订单加载失败，请稍后重试' }));
    });
  }, [client, scopeController]);

  // R3-09：重新发起支付——渠道失败/无链接的 pending 订单是支付死胡同（刷新
  // 永远拿同一行），唯一出路是新报价新订单：清空订单与报价引用后重跑加载
  // 流程（服务端已保证无链接 pending 不阻塞新结账）。
  const restartCheckout = useCallback((): void => {
    orderIdRef.current = null;
    quoteRef.current = null;
    setState({ status: 'loading' });
    setRetryToken((value) => value + 1);
  }, []);

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
            <Button type="button" onClick={() => setRetryToken((value) => value + 1)}>重试（同一报价服务端幂等；报价失效时自动重新报价）</Button>
          </>
        ) : null}
        {state.status === 'ready' ? (
          <>
            <section aria-live="polite">
              <h2>{spaceName} 的订单</h2>
              <p>{orderMessage(state.order)}</p>
              {/* AC1（#82）：三态产品状态由 purchase 投影驱动（待付款 → 已付款
                  待激活 → 已生效）；order.payment 是渠道面兜底。 */}
              <Status>{purchaseStateMessage(state.purchase, state.order)}</Status>
              {/* (审查 H1) 支付渠道单选：支付宝为 #82 主链默认；微信路径保持
                  可选（#81 形状）。选择只在下一单提交时生效。 */}
              <fieldset style={{ border: '1px solid #e7e7ea', borderRadius: 8, margin: '12px 0', padding: '8px 12px' }}>
                <legend>支付渠道</legend>
                <label>
                  <input type="radio" name="payment-channel" value="alipay"
                    checked={channel === 'alipay'}
                    onChange={() => { channelRef.current = 'alipay'; setChannel('alipay'); }} />
                  支付宝
                </label>
                <label>
                  <input type="radio" name="payment-channel" value="wechat"
                    checked={channel === 'wechat'}
                    onChange={() => { channelRef.current = 'wechat'; setChannel('wechat'); }} />
                  微信支付
                </label>
              </fieldset>
              {/* (审查 H1) 渠道切换入口：待付款订单上切换渠道是显式动作——
                  「改用 X 重新发起支付」走新报价新订单，绝不静默开第二单。
                  (D15-c) submittedChannel 为 null（深链打开、本页未提交过）时
                  不显示失配横幅——本页没有臆测过任何渠道。 */}
              {submittedChannel !== null && state.order.payment === 'pending' && channel !== submittedChannel ? (
                <>
                  <Status tone="error">当前订单以 {submittedChannel === 'alipay' ? '支付宝' : '微信支付'} 创建</Status>
                  <Button type="button" onClick={() => { restartCheckout(); }}>
                    改用{channel === 'alipay' ? '支付宝' : '微信支付'}重新发起支付（获取新报价）
                  </Button>
                </>
              ) : null}
              {/* 渠道支付入口（审查 F2）：渠道请求创建后展示跳转链接，用户由此完成支付。
                  R1-V13 / (OCR r4)：渲染前经 scheme 白名单校验，危险 scheme 一律不渲染；
                  checkoutHref 是唯一事实源，两处判断共用。
                  (D15-f) 已取消的购买不再引导付款——支付入口隐藏。 */}
              {(() => {
                if (state.purchase?.state === 'canceled') return null;
                const checkoutHref = state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url)
                  ? state.order.checkout_url : null;
                return checkoutHref ? (
                  <p>
                    <a href={checkoutHref} target="_blank" rel="noreferrer">前往支付</a>
                  </p>
                ) : null;
              })()}
              {/* R3-09：渠道创建失败（后端 202 姿态）——订单 pending 但没有任何
                  可用支付链接，本页刷新永远拿回同一行：给出渠道异常提示与
                  「重新发起支付」入口（新报价新订单），不让用户困在死胡同。 */}
              {state.order.payment === 'pending'
                && !(state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url)) ? (
                <>
                  <Status tone="error">支付渠道异常，此订单暂无可用支付链接</Status>
                  <Button type="button" onClick={restartCheckout}>重新发起支付（获取新报价）</Button>
                </>
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
