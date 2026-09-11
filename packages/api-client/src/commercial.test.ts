import assert from 'node:assert/strict';
import test from 'node:test';

import { createWeKnoraClient } from './client.ts';
import { createCommercialApi } from './commercial.ts';
import { ApiError } from './errors.ts';
import type { HttpTransport } from './ports.ts';

const order = { id: 'o1', payment: 'paid', fulfillment: 'pending', amount_fen: '100', currency: 'CNY' };
const summaryPayload = { plan_name: 'Team', paid_until: null, available: '1000', held: '0', refund_locked: '0', as_of: 't', stale: false };
const quotePayload = { id: 'q1', amount_fen: '9900', credit_delta: '500', expires_at: 't' };

function fakeApi(handler: (input: { method: string; path: string; body?: unknown }) => unknown, log: unknown[] = []) {
  return createCommercialApi(async (input) => {
    log.push({ method: input.method, path: input.path, body: input.body, signal: input.signal });
    return handler(input);
  });
}

test('maps each commercial endpoint to method, path, and body', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = fakeApi((input) => {
    if (input.path.endsWith('/summary')) return { success: true, data: summaryPayload };
    if (input.path.endsWith('/quotes')) return { success: true, data: quotePayload };
    if (input.path.endsWith('/refunds')) return { success: true, data: { id: 'r1', state: 'pending' } };
    return { success: true, data: order };
  }, requests);
  await api.getOrder('o1');
  await api.summary();
  await api.quote({ plan_key: 'team', plan_version: 1, subscription_version: 2 });
  await api.createOrder({ quote_id: 'q1', provider: 'wechat', idempotency_key: 'k1' });
  await api.requestRefund({ order_id: 'o1', amount_fen: '100', reason: 'dup', idempotency_key: 'k2' });
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/commercial/orders/o1', body: undefined, signal: undefined },
    { method: 'GET', path: '/api/v1/commercial/summary', body: undefined, signal: undefined },
    { method: 'POST', path: '/api/v1/commercial/quotes', body: { plan_key: 'team', plan_version: 1, subscription_version: 2 }, signal: undefined },
    { method: 'POST', path: '/api/v1/commercial/orders', body: { quote_id: 'q1', provider: 'wechat', idempotency_key: 'k1' }, signal: undefined },
    { method: 'POST', path: '/api/v1/commercial/refunds', body: { order_id: 'o1', amount_fen: '100', reason: 'dup', idempotency_key: 'k2' }, signal: undefined },
  ]);
});

test('unwraps the envelope before parsing and keeps amounts as strings', async () => {
  const api = fakeApi((input) => {
    if (input.path.endsWith('/summary')) return { success: true, data: summaryPayload };
    if (input.path.endsWith('/quotes')) return { success: true, data: quotePayload };
    if (input.path.endsWith('/refunds')) return { success: true, data: { id: 'r1', state: 'pending' } };
    return { success: true, data: order };
  });
  const value = await api.getOrder('o1');
  assert.equal(typeof value.amount_fen, 'string');
  assert.equal(value.amount_fen, '100');
  const summary = await api.summary();
  assert.equal(typeof summary.available, 'string');
  const quote = await api.quote({ plan_key: 'team', plan_version: 1, subscription_version: 2 });
  assert.equal(typeof quote.amount_fen, 'string');
  const refund = await api.requestRefund({ order_id: 'o1', amount_fen: '100', reason: 'dup', idempotency_key: 'k2' });
  assert.deepEqual(refund, { id: 'r1', state: 'pending' });
});

test('write inputs never carry tenant_id', async () => {
  const requests: Array<{ body?: unknown }> = [];
  const api = fakeApi((input) => {
    if (input.path.endsWith('/quotes')) return { success: true, data: quotePayload };
    if (input.path.endsWith('/refunds')) return { success: true, data: { id: 'r1', state: 'pending' } };
    return { success: true, data: order };
  }, requests);
  await api.quote({ plan_key: 'team', plan_version: 1, subscription_version: 2 });
  await api.createOrder({ quote_id: 'q1', provider: 'alipay', idempotency_key: 'k1' });
  await api.requestRefund({ order_id: 'o1', amount_fen: '100', reason: 'dup', idempotency_key: 'k2' });
  for (const request of requests) {
    assert.equal('tenant_id' in (request.body as Record<string, unknown>), false);
  }
});

test('failed envelopes surface as typed ApiError', async () => {
  const api = fakeApi(() => ({ success: false, error: { code: 'QUOTE_EXPIRED', message: 'quote expired' } }));
  await assert.rejects(
    api.quote({ plan_key: 'team', plan_version: 1, subscription_version: 2 }),
    (error: unknown) => error instanceof ApiError && error.code === 'QUOTE_EXPIRED' && error.message === 'quote expired',
  );
  const opaque = fakeApi(() => ({ success: false }));
  await assert.rejects(
    opaque.summary(),
    (error: unknown) => error instanceof ApiError && typeof error.code === 'string' && error.code.length > 0,
  );
});

test('rejects malformed payloads after unwrapping', async () => {
  const api = fakeApi(() => ({ success: true, data: { ...order, amount_fen: 100 } }));
  await assert.rejects(api.getOrder('o1'), /invalid order/);
  const missingData = fakeApi(() => ({ success: true }));
  await assert.rejects(missingData.summary(), (error: unknown) => error instanceof ApiError);
});

test('forwards abort signals on reads and writes', async () => {
  const seen: Array<AbortSignal | undefined> = [];
  const api = createCommercialApi(async (input) => {
    seen.push(input.signal);
    return { success: true, data: order };
  });
  const controller = new AbortController();
  await api.getOrder('o1', controller.signal);
  await api.createOrder({ quote_id: 'q1', provider: 'wechat', idempotency_key: 'k1' }, controller.signal);
  assert.equal(seen[0], controller.signal);
  assert.equal(seen[1], controller.signal);
});

test('exposes the commercial api through the client factory with transport-level cancellation', async () => {
  const sent: Array<{ method: string; url: string; signal?: AbortSignal }> = [];
  const transport: HttpTransport = {
    async send(request) {
      sent.push({ method: request.method, url: request.url, signal: request.signal });
      return { status: 200, headers: {}, body: { success: true, data: order } };
    },
  };
  const client = createWeKnoraClient({ baseURL: 'https://example.test/api', transport });
  const value = await client.commercial.getOrder('o1');
  assert.equal(value.id, 'o1');
  assert.equal(value.fulfillment, 'pending');
  assert.equal(sent[0]?.method, 'GET');
  assert.equal(sent[0]?.url, 'https://example.test/api/api/v1/commercial/orders/o1');
  await assert.rejects(
    client.commercial.getOrder('o1', AbortSignal.abort()),
    (error: unknown) => error instanceof ApiError && error.code === 'CANCELLED',
  );
});
