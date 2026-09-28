// SP14 Task 1 — GeneralPreferencesPanel 套餐卡片纯函数测试：文案组装。
// 合同修复后（commercial summary 真实形状）：套餐行显示 subscription.plan_key，
// base_tier 空间回退 base_tier_key 或本地化 billing.baseTier；到期行
// subscription.paid_until 透传，空/null 落 billing.noExpiry。额度三元组
// （available/held/refund_locked）后端无端点提供，已从卡片删除。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

// packages/ui 旧栈 pulls in theme.css; node:test needs the same short-circuit as
// the other settings panel tests (usage-panel.test.tsx pattern).
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => void }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

import type { CommercialSummary } from '@weknora/contracts';
const { formatBillingSummary } = await import('./GeneralPreferencesPanel.tsx');

function summary(overrides: Partial<CommercialSummary> = {}): CommercialSummary {
  return {
    tenant_id: 101,
    subscription: { id: 'sub-a', plan_key: 'pro', plan_version: 3, paid_until: '2027-09-20', version: 7 },
    base_tier: false,
    can_manage_billing: true,
    ...overrides,
  };
}

test('formatBillingSummary keeps the subscribed plan_key verbatim and passes paid_until through', () => {
  const zh = formatBillingSummary('zh-CN', summary({ subscription: { id: 'sub-a', plan_key: 'professional', plan_version: 2, paid_until: '2027-09-20', version: 1 } }));
  assert.equal(zh.plan, 'professional');
  assert.equal(zh.paidUntil, '2027-09-20', 'a non-null paid_until passes through verbatim');
});

test('formatBillingSummary falls back to the localized no-expiry note when paid_until is null or empty', () => {
  const zh = formatBillingSummary('zh-CN', summary({ subscription: { id: 'sub-a', plan_key: 'pro', plan_version: 3, paid_until: null, version: 7 } }));
  const en = formatBillingSummary('en-US', summary({ subscription: { id: 'sub-a', plan_key: 'pro', plan_version: 3, paid_until: '', version: 7 } }));
  assert.equal(zh.paidUntil, '无固定到期（未订阅）');
  assert.equal(en.paidUntil, 'No fixed expiry (not subscribed)');
});

test('formatBillingSummary shows the base tier key (or the localized label) for spaces without a subscription', () => {
  const withKey = formatBillingSummary('zh-CN', summary({ tenant_id: 303, subscription: null, base_tier: true, base_tier_key: 'free', can_manage_billing: true }));
  assert.equal(withKey.plan, 'free');
  const withoutKey = formatBillingSummary('zh-CN', summary({ tenant_id: 303, subscription: null, base_tier: true, can_manage_billing: true }));
  assert.equal(withoutKey.plan, '基础版');
  const withoutKeyEn = formatBillingSummary('en-US', summary({ tenant_id: 303, subscription: null, base_tier: true, can_manage_billing: true }));
  assert.equal(withoutKeyEn.plan, 'Base tier');
  // Base-tier spaces carry no paid_until: the no-expiry note applies too.
  assert.equal(withoutKey.paidUntil, '无固定到期（未订阅）');
});
