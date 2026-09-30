// 商业账单页纯函数测试：summary/usage loader（client.commercial 门面）与
// planDisplayName（合同修复后的真实形状：subscription.plan_key 或 base_tier
// 回退 base_tier_key/「基础版」）。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

// BillingPage.tsx now imports tdesign-react (whose barrel pulls component
// css); node:test needs the same short-circuit as the settings panel tests
// (usage-panel.test.tsx pattern).
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => void }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

import type { CommercialAccountCredits, CommercialSummary, CommercialUsageRow } from '@weknora/contracts';
const { loadCommercialSummary, loadCommercialUsage, loadCommercialAccount, planDisplayName } = await import('./BillingPage.tsx');
const { parseCommercialAccountCredits } = await import('@weknora/contracts');

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

// ---- #86 Task 5: the credits breakdown loader + contract parser ----

const breakdown: CommercialAccountCredits = {
  balance_micro: '1000000',
  held_micro: '200000',
  refund_locked_micro: '0',
  available_micro: '800000',
  projected_at: '2026-09-28T00:00:00Z',
  batches: [
    { source: 'monthly', period: '2026-09', granted_at: '2026-09-01T00:00:00Z', balance_micro: '1000000', expires_at: '2026-10-01T00:00:00Z' },
  ],
};

test('loadCommercialAccount maps breakdown and pending degradation', async () => {
  const ok = { commercial: { account: async () => breakdown } };
  const st = await loadCommercialAccount(ok.commercial);
  assert.equal(st.status, 'success');
  if (st.status === 'success' && st.credits) {
    assert.equal(st.credits.available_micro, '800000');
    assert.equal(st.credits.held_micro, '200000');
    assert.equal(st.credits.batches[0]?.source, 'monthly');
  } else {
    assert.fail('breakdown must map');
  }

  // A pending chain (benefits absent) answers null credits — the card
  // hides, never errors.
  const pending = { commercial: { account: async () => null } };
  const pd = await loadCommercialAccount(pending.commercial);
  assert.equal(pd.status, 'success');
  if (pd.status === 'success') assert.equal(pd.credits, null);

  const failing = { commercial: { account: async () => { throw new Error('NETWORK'); } } };
  const fe = await loadCommercialAccount(failing.commercial);
  assert.equal(fe.status, 'error');
  if (fe.status === 'error') assert.equal(fe.message, 'NETWORK');
});

test('parseCommercialAccountCredits rejects malformed digit strings', () => {
  const bad = { ...breakdown, balance_micro: '12.5' };
  assert.throws(() => parseCommercialAccountCredits(bad));
  assert.throws(() => parseCommercialAccountCredits({ ...breakdown, held_micro: 'abc' }));
  assert.throws(() => parseCommercialAccountCredits(null));
  assert.throws(() => parseCommercialAccountCredits('nope'));
  // A well-formed answer parses verbatim.
  const parsed = parseCommercialAccountCredits(breakdown);
  assert.equal(parsed.available_micro, '800000');
  assert.equal(parsed.batches.length, 1);
});

// ---- OCR r1 round (CR-86-1 / CR-86-2) ----

// CR-86-2: available_micro is the one SIGNED face — over-committed holds
// surface a negative availability honestly (plan Task 4: "可为负数如实显示
// ——超占即事实"); a negative digit string must parse, never throw.
test('parseCommercialAccountCredits accepts a negative available_micro', () => {
  const overcommitted = { ...breakdown, available_micro: '-200000' };
  const parsed = parseCommercialAccountCredits(overcommitted);
  assert.equal(parsed.available_micro, '-200000');
  // Malformed signed values still reject.
  assert.throws(() => parseCommercialAccountCredits({ ...breakdown, available_micro: '-' }));
  assert.throws(() => parseCommercialAccountCredits({ ...breakdown, available_micro: '-12.5' }));
  // The non-negative faces keep rejecting negatives.
  assert.throws(() => parseCommercialAccountCredits({ ...breakdown, balance_micro: '-1' }));
  assert.throws(() => parseCommercialAccountCredits({ ...breakdown, held_micro: '-1' }));
});

// CR-86-1: a batch whose granted_at the backend omitted (the cross-month
// lingering-batch shape — a registry batch the terminated authority wallet
// no longer backs) must NOT kill the whole breakdown: the display-only
// field degrades to '' instead of throwing.
test('parseCommercialAccountCredits degrades a missing granted_at to empty string', () => {
  const legacy = {
    ...breakdown,
    batches: [
      { source: 'monthly', period: '2026-08', balance_micro: '0', expires_at: '2026-09-01T00:00:00Z' },
    ],
  };
  const parsed = parseCommercialAccountCredits(legacy);
  assert.equal(parsed.batches[0]?.granted_at, '');
  // A present granted_at still passes through verbatim.
  const full = parseCommercialAccountCredits(breakdown);
  assert.equal(full.batches[0]?.granted_at, '2026-09-01T00:00:00Z');
});
