// 商业账单页纯函数测试：summary/usage loader（client.commercial 门面）与
// planDisplayName（合同修复后的真实形状：subscription.plan_key 或 base_tier
// 回退 base_tier_key/「基础版」）。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

// @weknora/ui pulls in theme.css; node:test needs the same short-circuit as
// the settings panel tests (usage-panel.test.tsx pattern).
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => void }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

import type { CommercialSummary, CommercialUsageRow } from '@weknora/contracts';
const { loadCommercialSummary, loadCommercialUsage, planDisplayName } = await import('./BillingPage.tsx');

function client(summaryOrError: () => Promise<CommercialSummary>, usageOrError: () => Promise<CommercialUsageRow[]>) {
  return {
    commercial: {
      summary: () => summaryOrError(),
      usage: () => usageOrError(),
    },
  };
}

const subscribed: CommercialSummary = {
  tenant_id: 101,
  subscription: { id: 'sub-a', plan_key: 'pro', plan_version: 3, paid_until: '2027-01-01T00:00:00Z', version: 7 },
  base_tier: false,
  can_manage_billing: true,
};

test('loadCommercialSummary/loadCommercialUsage map success and error outcomes', async () => {
  const ok = client(async () => subscribed, async () => [{ resource: 'storage_files', used: 7, limit: 10 }, { resource: 'wiki_pages', used: 3, limit: null }]);
  const summaryState = await loadCommercialSummary(ok.commercial);
  assert.equal(summaryState.status, 'success');
  const usageState = await loadCommercialUsage(ok.commercial);
  assert.equal(usageState.status, 'success');
  if (usageState.status === 'success') {
    assert.equal(usageState.rows.length, 2);
    assert.equal(usageState.rows[1]?.limit, null);
  }

  const failing = client(async () => { throw new Error('INVALID_RESPONSE'); }, async () => { throw new Error('boom'); });
  const summaryError = await loadCommercialSummary(failing.commercial);
  assert.equal(summaryError.status, 'error');
  if (summaryError.status === 'error') assert.equal(summaryError.message, 'INVALID_RESPONSE');
  const usageError = await loadCommercialUsage(failing.commercial);
  assert.equal(usageError.status, 'error');
  if (usageError.status === 'error') assert.equal(usageError.message, 'boom');
});

test('planDisplayName shows plan_key for subscribed spaces and the base-tier fallback otherwise', () => {
  assert.equal(planDisplayName(subscribed), 'pro');
  assert.equal(planDisplayName({ tenant_id: 303, subscription: null, base_tier: true, base_tier_key: 'free', can_manage_billing: true }), 'free');
  assert.equal(planDisplayName({ tenant_id: 303, subscription: null, base_tier: true, can_manage_billing: true }), '基础版');
});
