import assert from 'node:assert/strict';
import test from 'node:test';

import { parseCommercialSummary, parseCommercialUsageList, parseOrderView, parseQuoteView, parseRefundView } from '../src/commercial.ts';

const order = { id: 'o1', payment: 'paid', fulfillment: 'pending', amount_fen: '100', currency: 'CNY' };

test('paid is distinct from fulfilled', () => {
  const value = parseOrderView({id:'o1',payment:'paid',fulfillment:'pending',amount_fen:'100',currency:'CNY'});
  assert.equal(value.fulfillment, 'pending');
  assert.throws(() => parseOrderView({...value, amount_fen:100}));
});

test('every payment and fulfillment state round-trips independently', () => {
  const payments = ['pending', 'paid', 'closed'] as const;
  const fulfillments = ['pending', 'processing', 'fulfilled', 'attention'] as const;
  for (const payment of payments) {
    for (const fulfillment of fulfillments) {
      const value = parseOrderView({ id: 'o1', payment, fulfillment, amount_fen: '0', currency: 'CNY' });
      assert.equal(value.payment, payment);
      assert.equal(value.fulfillment, fulfillment);
      assert.equal(value.amount_fen, '0');
    }
  }
});

test('rejects malformed orders', () => {
  const malformed: unknown[] = [
    null,
    'order',
    42,
    [order],
    { payment: 'paid', fulfillment: 'pending', amount_fen: '100', currency: 'CNY' },
    { ...order, id: 42 },
    { ...order, amount_fen: 100 },
    { ...order, amount_fen: '-100' },
    { ...order, amount_fen: '10.5' },
    { ...order, amount_fen: 'abc' },
    { ...order, amount_fen: '' },
    { ...order, currency: 'USD' },
    { ...order, payment: 'refunded' },
    { ...order, fulfillment: 'shipped' },
  ];
  for (const value of malformed) assert.throws(() => parseOrderView(value), /invalid order/);
});

// The summary wire shape is the handler's real projection
// (internal/handler/commercial.go Summary): tenant_id + subscription|null +
// base_tier flags — NOT a ledger (available/held/refund_locked are served by
// no endpoint and were a fabrication the parser rejected the real payload on).
const subscribedSummary = {
  tenant_id: 101,
  subscription: {
    id: 'sub-a',
    plan_key: 'pro',
    plan_version: 3,
    paid_until: '2027-01-01T00:00:00Z',
    version: 7,
    downgrade_reason: '',
  },
  base_tier: false,
  can_manage_billing: true,
};

const baseTierSummary = {
  tenant_id: 303,
  subscription: null,
  base_tier: true,
  base_tier_key: 'free',
  can_manage_billing: false,
};

test('parses a subscribed commercial summary field by field', () => {
  const value = parseCommercialSummary(subscribedSummary);
  assert.equal(value.tenant_id, 101);
  assert.equal(value.base_tier, false);
  assert.equal(value.can_manage_billing, true);
  assert.equal(value.subscription?.id, 'sub-a');
  assert.equal(value.subscription?.plan_key, 'pro');
  assert.equal(value.subscription?.plan_version, 3);
  assert.equal(value.subscription?.paid_until, '2027-01-01T00:00:00Z');
  assert.equal(value.subscription?.version, 7);
  assert.equal(value.subscription?.downgrade_reason, '');
  // Optional downgrade_reason may be absent entirely (base branch fields
  // like base_tier_key likewise: unknown fields pass through verbatim).
  const minimal = { tenant_id: 1, subscription: { id: 's', plan_key: 'pro', plan_version: 1, paid_until: null, version: 1 }, base_tier: false, can_manage_billing: true };
  const parsed = parseCommercialSummary(minimal);
  assert.equal(parsed.subscription?.paid_until, null);
  assert.equal(parsed.subscription?.downgrade_reason, undefined);
});

test('parses a base-tier summary with a null subscription', () => {
  const value = parseCommercialSummary(baseTierSummary);
  assert.equal(value.tenant_id, 303);
  assert.equal(value.subscription, null);
  assert.equal(value.base_tier, true);
  assert.equal(value.base_tier_key, 'free');
  assert.equal(value.can_manage_billing, false);
});

test('rejects malformed commercial summaries', () => {
  const malformed: unknown[] = [
    null,
    'summary',
    42,
    [subscribedSummary],
    { ...subscribedSummary, tenant_id: '101' },
    { ...subscribedSummary, base_tier: 'no' },
    { ...subscribedSummary, can_manage_billing: 1 },
    { ...subscribedSummary, subscription: 'pro' },
    { ...subscribedSummary, subscription: [] },
    // subscription branch field violations
    { ...subscribedSummary, subscription: { ...subscribedSummary.subscription, id: 42 } },
    { ...subscribedSummary, subscription: { ...subscribedSummary.subscription, plan_key: '' } },
    { ...subscribedSummary, subscription: { ...subscribedSummary.subscription, plan_version: '3' } },
    { ...subscribedSummary, subscription: { ...subscribedSummary.subscription, version: 7.5 } },
    { ...subscribedSummary, subscription: { ...subscribedSummary.subscription, paid_until: 20270101 } },
    { ...subscribedSummary, subscription: { ...subscribedSummary.subscription, downgrade_reason: 42 } },
    // base branch field violations
    { ...baseTierSummary, base_tier_key: 42 },
    // missing core fields
    { subscription: null, base_tier: true, can_manage_billing: true },
    { tenant_id: 1, base_tier: true, can_manage_billing: true },
    { tenant_id: 1, subscription: null, can_manage_billing: true },
    { tenant_id: 1, subscription: null, base_tier: true },
  ];
  for (const value of malformed) assert.throws(() => parseCommercialSummary(value), /invalid commercial summary/);
});

test('passes unknown summary fields through verbatim (contract convention)', () => {
  const value = parseCommercialSummary({ ...subscribedSummary, future_field: 'x' }) as Record<string, unknown>;
  assert.equal(value.future_field, 'x');
});

// Usage rows mirror GET /api/v1/commercial/usage data: {resource, used,
// limit} with limit null for an unlimited dimension.
const usageRows = [
  { resource: 'storage_files', used: 7, limit: 10 },
  { resource: 'wiki_pages', used: 3, limit: null },
];

test('parses commercial usage rows including null limits', () => {
  const value = parseCommercialUsageList(usageRows);
  assert.equal(value.length, 2);
  assert.deepEqual(value[0], { resource: 'storage_files', used: 7, limit: 10 });
  assert.deepEqual(value[1], { resource: 'wiki_pages', used: 3, limit: null });
  assert.deepEqual(parseCommercialUsageList([]), []);
});

test('rejects malformed commercial usage lists', () => {
  const malformed: unknown[] = [
    null,
    'usage',
    {},
    { data: usageRows },
    [['storage_files', 7, 10]],
    [{ ...usageRows[0], resource: '' }],
    [{ ...usageRows[0], resource: 42 }],
    [{ ...usageRows[0], used: '7' }],
    [{ ...usageRows[0], limit: '10' }],
    [{ ...usageRows[0], limit: undefined }],
    [{ resource: 'r' }],
    [{ used: 1, limit: null }],
  ];
  for (const value of malformed) assert.throws(() => parseCommercialUsageList(value), /invalid commercial usage/);
});

const quote = { id: 'q1', amount_fen: '9900', credit_delta: '500', expires_at: '2026-09-12T00:00:00Z' };

test('parses quote views', () => {
  const value = parseQuoteView(quote);
  assert.equal(value.id, 'q1');
  assert.equal(value.amount_fen, '9900');
  assert.equal(value.credit_delta, '500');
  assert.equal(value.expires_at, '2026-09-12T00:00:00Z');
  assert.equal(parseQuoteView({ ...quote, credit_delta: '-500' }).credit_delta, '-500');
});

test('rejects malformed quote views', () => {
  const malformed: unknown[] = [
    null,
    'quote',
    { ...quote, id: 42 },
    { ...quote, amount_fen: 9900 },
    { ...quote, amount_fen: '-1' },
    { ...quote, credit_delta: 500 },
    { ...quote, expires_at: '' },
  ];
  for (const value of malformed) assert.throws(() => parseQuoteView(value), /invalid quote/);
});

const refund = { id: 'r1', state: 'revocation_pending', amount_fen: '9900', locked_credits: '500' };

test('parses refund views field by field across every C05 state', () => {
  // The wire vocabulary is exactly internal/commercial/refund.go's lifecycle.
  const states = [
    'requested',
    'reviewing',
    'pending',
    'revocation_pending',
    'completed',
    'failed_confirmed',
    'not_created_confirmed',
  ];
  for (const state of states) {
    const value = parseRefundView({ ...refund, state });
    assert.equal(value.id, 'r1');
    assert.equal(value.state, state);
    assert.equal(value.amount_fen, '9900');
    assert.equal(value.locked_credits, '500');
  }
  assert.equal(parseRefundView({ ...refund, amount_fen: '0', locked_credits: '0' }).amount_fen, '0');
});

test('rejects malformed refund views', () => {
  const malformed: unknown[] = [
    null,
    'refund',
    42,
    [refund],
    { state: 'requested', amount_fen: '9900', locked_credits: '500' },
    { ...refund, id: 42 },
    { ...refund, id: '' },
    { ...refund, amount_fen: 9900 },
    { ...refund, amount_fen: '-1' },
    { ...refund, amount_fen: '99.5' },
    // C05: locked credits are held credits and are never negative — a release is
    // represented by a lower value (mirrors CommercialSummary.refund_locked).
    { ...refund, locked_credits: -1 },
    { ...refund, locked_credits: '-1' },
    { ...refund, locked_credits: '1.5' },
    // Display-level fallbacks are not wire states: the server projection
    // vocabulary is refund.go's, so these must be rejected on parse.
    { ...refund, state: 'refund_unknown' },
    { ...refund, state: 'rejected' },
    { ...refund, state: 'refunded' },
    { ...refund, state: 42 },
  ];
  for (const value of malformed) assert.throws(() => parseRefundView(value), /invalid refund/);
});
