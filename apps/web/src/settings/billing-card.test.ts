// SP14 Task 1 — GeneralPreferencesPanel 套餐卡片纯函数测试：文案组装
// （plan_name 原样、paid_until null 落 billing.noExpiry、微积分整数本地化）。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

// @weknora/ui pulls in theme.css; node:test needs the same short-circuit as
// the other settings panel tests (usage-panel.test.tsx pattern).
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

import type { CommercialSummary } from '@weknora/contracts';
const { formatBillingSummary } = await import('./GeneralPreferencesPanel.tsx');

function summary(overrides: Partial<CommercialSummary> = {}): CommercialSummary {
  return {
    plan_name: 'pro',
    paid_until: '2027-09-20',
    available: '1234567',
    held: '0',
    refund_locked: '0',
    as_of: '2026-09-20T00:00:00Z',
    stale: false,
    ...overrides,
  };
}

test('formatBillingSummary keeps the plan name verbatim and localizes microcredit integers', () => {
  const zh = formatBillingSummary('zh-CN', summary({ plan_name: '专业版', available: '1234567', held: '1000' }));
  assert.equal(zh.plan, '专业版');
  assert.equal(zh.available, '1,234,567');
  assert.equal(zh.held, '1,000');
  assert.equal(zh.paidUntil, '2027-09-20', 'a non-null paid_until passes through verbatim');

  const en = formatBillingSummary('en-US', summary({ available: '42' }));
  assert.equal(en.available, '42');
  assert.equal(en.held, '0');
});

test('formatBillingSummary falls back to the localized no-expiry note when paid_until is null', () => {
  const zh = formatBillingSummary('zh-CN', summary({ paid_until: null }));
  const en = formatBillingSummary('en-US', summary({ paid_until: null }));
  assert.equal(zh.paidUntil, '无固定到期（未订阅）');
  assert.equal(en.paidUntil, 'No fixed expiry (not subscribed)');
});
