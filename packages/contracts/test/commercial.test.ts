import assert from 'node:assert/strict';
import test from 'node:test';

import { parseCommercialSummary, parseOrderView, parseQuoteView } from '../src/commercial.ts';

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

const summary = {
  plan_name: 'Team',
  paid_until: null as string | null,
  available: '1000',
  held: '50',
  refund_locked: '25',
  as_of: '2026-09-11T00:00:00Z',
  stale: false,
};

test('parses commercial summaries field by field', () => {
  const value = parseCommercialSummary(summary);
  assert.equal(value.plan_name, 'Team');
  assert.equal(value.paid_until, null);
  assert.equal(value.available, '1000');
  assert.equal(value.held, '50');
  assert.equal(value.refund_locked, '25');
  assert.equal(value.as_of, '2026-09-11T00:00:00Z');
  assert.equal(value.stale, false);
  const renewed = parseCommercialSummary({ ...summary, paid_until: '2026-12-31', stale: true });
  assert.equal(renewed.paid_until, '2026-12-31');
  assert.equal(renewed.stale, true);
});

test('rejects malformed commercial summaries', () => {
  const malformed: unknown[] = [
    null,
    'summary',
    { ...summary, plan_name: 42 },
    { ...summary, paid_until: 20261231 },
    { ...summary, available: 1000 },
    { ...summary, available: '-1' },
    { ...summary, held: '1.5' },
    { ...summary, refund_locked: 'abc' },
    { ...summary, as_of: null },
    { ...summary, stale: 'no' },
    { available: '1000', held: '0', refund_locked: '0', as_of: 't', stale: false },
  ];
  for (const value of malformed) assert.throws(() => parseCommercialSummary(value), /invalid commercial summary/);
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
