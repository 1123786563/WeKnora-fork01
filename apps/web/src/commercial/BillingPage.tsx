import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { CommercialAccountView, CommercialSummary, CommercialUsageRow, OrderView, PurchaseView } from '@weknora/contracts';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button } from 'tdesign-react';
import { Card, Status } from './ui.tsx';
// (D15-f) 共享购买状态词表：本页不再持有私有的三行内联文案。
import { PURCHASE_STATE_LABEL } from './order-state.ts';

export type CommercialSummaryState =
  | { status: 'success'; summary: CommercialSummary }
  | { status: 'error'; message: string };

export type CommercialUsageState =
  | { status: 'success'; rows: CommercialUsageRow[] }
  | { status: 'error'; message: string };

// #86/#100：账务信封状态——state=linked|pending（pending 即 spec L169 的
// 「等待账务同步」稳定产品状态），credits 为 null 表示 benefits 尚未就绪，
// 卡片隐藏而非报错，状态行照常渲染。
export type CommercialAccountState =
  | { status: 'success'; view: CommercialAccountView }
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

export async function loadCommercialAccount(
  client: Pick<WeKnoraClient['commercial'], 'account'>,
  signal?: AbortSignal,
): Promise<CommercialAccountState> {
  try {
    return { status: 'success', view: await client.account(signal) };
  } catch (error) {
    if (signal?.aborted) throw error;
    return { status: 'error', message: error instanceof Error ? error.message : 'Unable to load credits breakdown' };
  }
}

/** #100 AC①：账务状态闭合中文文案——「等待账务同步」是 spec L169 十状态中
 * 原先无 UI 映射的最后一个（pending 原本只是卡片隐藏）。 */
export function accountStateLabel(view: Pick<CommercialAccountView, 'state'>): string {
  return view.state === 'pending' ? '等待账务同步' : '账务已连接';
}

/** #100 AC③：权限差异化提示——无账单管理权的调用者（服务端写门已 403）
 * 在账单页看到闭合提示；管理者返回 null（购买入口维持原样）。 */
export function billingManageNotice(canManageBilling: boolean): string | null {
  if (canManageBilling) return null;
  return '当前角色无账单管理权限：购买、变更与退款由空间所有者或获授权成员操作。';
}

/** micro → 显示两位小数（纯展示换算，不在契约层做）。 */
export function microToDisplay(micro: string): string {
  const n = Number(micro);
  if (!Number.isFinite(n)) return micro;
  return (n / 1_000_000).toFixed(2);
}

/** 批次来源的显示名：套餐月度 / 充值。 */
export function batchSourceLabel(source: string): string {
  return source === 'topup' ? '充值' : '套餐月度';
}

/** 批次是否在 30 天内到期（近到期行加标记）。已过期（exp < now）不算近到期：
 * 后端投影对过期 top-up 批次保留行（balance 置 0、ExpiresAt 为过去值仍输出），
 * 无下界时这些行会被负差值恒真地标成「近到期」而非呈现已过期事实
 * （OCR84-R1-16）。 */
export function batchExpiringWithin(expiresAt: string, now: Date = new Date()): boolean {
  const exp = new Date(expiresAt).getTime();
  if (!Number.isFinite(exp)) return false;
  const delta = exp - now.getTime();
  return delta >= 0 && delta <= 30 * 24 * 3600 * 1000;
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
  // #86：余额分解（pending → credits null，卡片隐藏）。
  const [accountState, setAccountState] = useState<CommercialAccountState>({ status: 'error', message: 'Loading…' });
  // #81：购买状态（awaiting_payment → 套餐行显示「待付款（权益未开放）」）。
  const [purchase, setPurchase] = useState<PurchaseView | null>(null);
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'commercial-summary'), [scope.scope]);
  // Space label comes from the current scope tenant; fall back to a neutral label
  // until the shell provides a tenant display name.
  const spaceName = scope.scope.tenantId ? `空间 ${scope.scope.tenantId}` : '当前空间';

  useEffect(() => {
    let active = true;
    setState({ status: 'error', message: 'Loading…' });
    setUsageState({ status: 'error', message: 'Loading…' });
    setAccountState({ status: 'error', message: 'Loading…' });
    // R1-V15：切换租户/重载时同步清掉上一租户的购买状态，避免 summary 先到、
    // purchaseStatus 后到期间套餐行短暂显示错误租户的「待付款」。
    setPurchase(null);
    void loadCommercialSummary(client.commercial, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setState(next);
    });
    void loadCommercialUsage(client.commercial, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setUsageState(next);
    });
    // #86：并行读取余额分解；失败静默降级（卡片隐藏，不阻塞账单页）。
    void loadCommercialAccount(client.commercial, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setAccountState(next);
    });
    // #81：并行读取购买状态；失败静默降级（待付款行只是缺席，不阻塞账单页）。
    void client.commercial.purchaseStatus(scope.signal).then((view) => {
      if (active && scopeController.isCurrent(scope.scope)) setPurchase(view);
    }).catch(() => {
      if (active && scopeController.isCurrent(scope.scope)) setPurchase(null);
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
          <>
          <ul className="wk-list" data-testid="billing-summary-list">
            <li>
              <strong>套餐</strong>
              <span>
                {planDisplayName(state.summary)}
                {/* AC1（#82 三态）：待付款（权益未开放）→ 已付款待激活 → 已生效。
                    (D15-f) 文案经共享词表 PURCHASE_STATE_LABEL——两页不再各持一套。 */}
                {purchase?.state && purchase.state !== 'absent'
                  ? ` · ${PURCHASE_STATE_LABEL[purchase.state]}`
                  : ''}
                {/* (#84 / AC4) 付款异常后缀两种形态：①待付款异常单（order 子对象的
                    fulfillment=attention——错额/部分/错币，订单无法正常推进）；
                    ②已生效但仍带未处置多收款异常的单（payment_attention 附加字段，
                    R4 裁决：多收款不改写用户主状态，只追加提示）。M1 类型裁决：
                    契约 OrderView 无 payment_attention 字段（#85 并行期契约零改动），
                    用局部类型断言读取加法字段——运行时由 parseOrderView 未知字段
                    透传保证在位。 */}
                {purchase?.order?.fulfillment === 'attention' ? ' · 付款异常（待处理）' : ''}
                {purchase?.state === 'active'
                  && (purchase.order as (OrderView & { payment_attention?: boolean }) | undefined)?.payment_attention
                  ? ' · 付款异常（待处理）'
                  : ''}
              </span>
            </li>
            <li><strong>到期</strong><span>{state.summary.subscription?.paid_until || '无固定到期（未订阅）'}</span></li>
          </ul>
          {/* #100 AC③：权限差异化——无账单管理权的调用者看到闭合提示（服务端
              写门已 403，这里只是 UI 面）；管理者不渲染。 */}
          {billingManageNotice(state.summary.can_manage_billing) !== null ? (
            <p className="wk-muted" data-testid="billing-manage-notice">{billingManageNotice(state.summary.can_manage_billing)}</p>
          ) : null}
          </>
        ) : null}
      </Card>
      {accountState.status === 'success' && accountState.view.credits ? (
        <Card>
          <h2>余额</h2>
          {/* #86：余额分解——总余额/预占/退款锁定/可用 + 批次表。 */}
          <ul className="wk-list" data-testid="billing-credits-breakdown">
            <li><strong>总余额</strong><span>{microToDisplay(accountState.view.credits.balance_micro)}</span></li>
            <li><strong>预占（进行中任务）</strong><span>{microToDisplay(accountState.view.credits.held_micro)}</span></li>
            <li><strong>退款锁定</strong><span>{microToDisplay(accountState.view.credits.refund_locked_micro)}</span></li>
            <li><strong>可用</strong><span>{microToDisplay(accountState.view.credits.available_micro)}</span></li>
          </ul>
          {accountState.view.credits.batches.length > 0 ? (
            <table className={USAGE_TABLE} data-testid="billing-credits-batches">
              <thead>
                <tr className="border-b border-[#e7e7ea]">
                  <th className={USAGE_TABLE_CELL + ' font-semibold'}>来源</th>
                  <th className={USAGE_TABLE_CELL + ' font-semibold'}>批次 / 发放日</th>
                  <th className={USAGE_TABLE_CELL + ' font-semibold'}>到期</th>
                  <th className={USAGE_NUMBER_CELL + ' font-semibold'}>余额</th>
                </tr>
              </thead>
              <tbody>
                {accountState.view.credits.batches.map((batch, index) => (
                  <tr key={`${batch.source}-${batch.period}-${index}`}
                    data-testid={batchExpiringWithin(batch.expires_at) ? 'batch-expiring' : undefined}>
                    <td className={USAGE_TABLE_CELL}>{batchSourceLabel(batch.source)}</td>
                    <td className={USAGE_TABLE_CELL}>
                      {batch.period === '' ? '充值批次' : batch.period}
                      <span className="wk-muted"> · {batch.granted_at.slice(0, 10)}</span>
                    </td>
                    <td className={USAGE_TABLE_CELL}>{batch.expires_at.slice(0, 10)}</td>
                    <td className={USAGE_NUMBER_CELL}>{microToDisplay(batch.balance_micro)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : null}
          <p className="wk-muted">对账时间：{accountState.view.credits.projected_at}</p>
        </Card>
      ) : accountState.status === 'success' && accountState.view.state === 'pending' ? (
        <Card>
          {/* #100 AC①：等待账务同步的 UI 映射——十状态的最后一个此前无渲染面。 */}
          <ul className="wk-list" data-testid="billing-account-state">
            <li>
              <strong>账务状态</strong>
              <span>
                {accountStateLabel(accountState.view)}
                {accountState.view.reason !== ''
                  ? <span className="wk-muted"> · {accountState.view.reason}</span>
                  : null}
              </span>
            </li>
          </ul>
          <p className="wk-muted">账单数据暂不可用；数据恢复后此处自动展示余额分解。</p>
        </Card>
      ) : accountState.status === 'error' && accountState.message !== 'Loading…' ? (
        <Card>
          <Status tone="error">{accountState.message}</Status>
        </Card>
      ) : null}
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
