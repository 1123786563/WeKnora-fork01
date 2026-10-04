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

import type { CommercialAccountCredits, CommercialAccountView, CommercialSummary, CommercialUsageRow } from '@weknora/contracts';
const { loadCommercialSummary, loadCommercialUsage, loadCommercialAccount, planDisplayName, accountStateLabel, billingManageNotice } = await import('./BillingPage.tsx');
const { batchExpiringWithin } = await import('./BillingPage.tsx');
const { parseCommercialAccountCredits, parseCommercialAccountView } = await import('@weknora/contracts');

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

// ---- #100 [Lago 28]: the account envelope (state + credits) and the
// two remaining Billing Center surfaces — waiting-for-sync state row and
// the can_manage_billing role notice ----

const linkedView: CommercialAccountView = {
  state: 'linked',
  reason: '',
  credits: breakdown,
};
const pendingView: CommercialAccountView = { state: 'pending', reason: 'unreachable', credits: null };

test('loadCommercialAccount maps the account envelope (linked + pending + error)', async () => {
  const ok = { commercial: { account: async () => linkedView } };
  const st = await loadCommercialAccount(ok.commercial);
  assert.equal(st.status, 'success');
  if (st.status === 'success' && st.view.credits) {
    assert.equal(st.view.state, 'linked');
    assert.equal(st.view.credits.available_micro, '800000');
    assert.equal(st.view.credits.held_micro, '200000');
    assert.equal(st.view.credits.batches[0]?.source, 'monthly');
  } else {
    assert.fail('linked envelope must map');
  }

  // A pending chain (benefits absent) answers null credits — the card
  // hides, never errors, and the state row still renders.
  const pending = { commercial: { account: async () => pendingView } };
  const pd = await loadCommercialAccount(pending.commercial);
  assert.equal(pd.status, 'success');
  if (pd.status === 'success') {
    assert.equal(pd.view.state, 'pending');
    assert.equal(pd.view.credits, null);
  }

  const failing = { commercial: { account: async () => { throw new Error('NETWORK'); } } };
  const fe = await loadCommercialAccount(failing.commercial);
  assert.equal(fe.status, 'error');
  if (fe.status === 'error') assert.equal(fe.message, 'NETWORK');
});

test('parseCommercialAccountView parses linked/pending and rejects open state vocabulary', () => {
  const wire = {
    state: 'linked',
    benefits: { credits: { balance_micro: '1000000', held_micro: '200000', refund_locked_micro: '0', available_micro: '800000', batches: [] } },
  };
  const parsed = parseCommercialAccountView(wire);
  assert.equal(parsed.state, 'linked');
  assert.equal(parsed.reason, '');
  assert.equal(parsed.credits?.available_micro, '800000');

  // The pending degrade: no benefits key → credits null, reason degrades
  // to '' when absent, never a throw.
  const pendingWire = { state: 'pending', reason: 'unreachable' };
  const pending = parseCommercialAccountView(pendingWire);
  assert.equal(pending.state, 'pending');
  assert.equal(pending.reason, 'unreachable');
  assert.equal(pending.credits, null);

  // The closed state set is enforced — a raw provider state rejects.
  assert.throws(() => parseCommercialAccountView({ state: 'active' }));
  assert.throws(() => parseCommercialAccountView(null));
});

// AC①: waiting for billing synchronization gains its UI mapping — the
// 10th stable product state (spec L169) becomes visible instead of the
// card silently hiding.
test('accountStateLabel maps the closed account states to stable Chinese copy', () => {
  assert.equal(accountStateLabel(pendingView), '等待账务同步');
  assert.equal(accountStateLabel(linkedView), '账务已连接');
});

// AC③: the role-differentiated operations surface — a caller without
// billing-management authority gets the closed notice, a manager gets
// none (the purchase entries stay as they are).
test('billingManageNotice differentiates non-managing callers only', () => {
  assert.equal(billingManageNotice(true), null);
  const notice = billingManageNotice(false);
  assert.ok(notice !== null && notice.includes('无账单管理权限'));
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

// ---- OCR84-R1-04 / R1-16 round ----

// R1-04: the backend benefitsWire emits a credits object even when the
// projection chain answered credits==nil — batches serializes as null and
// projected_at carries NO key. The parser must degrade both (projected_at
// '' like granted_at, null batches → empty array) so api-client account()
// succeeds and BillingPage hides the card instead of rendering the English
// parse-error card — the "credits absent → null card hidden, never an
// error" promise.
test('parseCommercialAccountCredits degrades the benefitsWire nil-chain shape (batches null, no projected_at)', () => {
  const degraded = {
    balance_micro: '0',
    held_micro: '0',
    refund_locked_micro: '0',
    available_micro: '0',
    batches: null,
  };
  const parsed = parseCommercialAccountCredits(degraded);
  assert.equal(parsed.projected_at, '');
  assert.equal(parsed.batches.length, 0);
  // The batches key entirely absent degrades the same way.
  const absent = { balance_micro: '0', held_micro: '0', refund_locked_micro: '0', available_micro: '0' };
  const parsed2 = parseCommercialAccountCredits(absent);
  assert.equal(parsed2.projected_at, '');
  assert.equal(parsed2.batches.length, 0);
  // A present projected_at still passes through verbatim; a present
  // non-array batches value still rejects (strictness kept for real
  // contract violations).
  assert.equal(parseCommercialAccountCredits(breakdown).projected_at, '2026-09-28T00:00:00Z');
  assert.throws(() => parseCommercialAccountCredits({ ...breakdown, batches: 'nope' }));
});

// R1-16: 已过期批次（exp < now，后端投影对过期 top-up 行保留 balance=0 的行）
// 不算「近到期」——负差值恒真的下界缺失会把过期行标成近到期。
test('batchExpiringWithin excludes already-expired batches and keeps the 30d window', () => {
  const now = new Date('2026-09-28T00:00:00Z');
  // 已过期（-1 天）：不算近到期。
  assert.equal(batchExpiringWithin('2026-09-27T00:00:00Z', now), false);
  // 恰好现在：界内（0 ≤ delta ≤ 30d）。
  assert.equal(batchExpiringWithin('2026-09-28T00:00:00Z', now), true);
  // 29 天后：近到期。
  assert.equal(batchExpiringWithin('2026-10-27T00:00:00Z', now), true);
  // 31 天后：不算。
  assert.equal(batchExpiringWithin('2026-10-29T00:00:00Z', now), false);
  // 不可解析：不算。
  assert.equal(batchExpiringWithin('not-a-date', now), false);
});
