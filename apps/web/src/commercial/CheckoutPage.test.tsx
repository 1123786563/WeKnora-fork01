import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (h: { resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/commercial/checkout' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot } = await import('react-dom/client');
type Root = import('react-dom/client').Root;

const quote = {
  id: 'qt_1', plan_key: 'pro', plan_version: 1, amount_fen: '9900',
  credit_delta: '9900000', expires_at: '2026-09-23T12:00:00Z',
  currency: 'CNY',
  features: { advanced_models: true },
  line_items: [{ kind: 'subscription_fee', name: 'Pro', amount_fen: '9900' }],
};
const order = {
  id: 'ord_1', quote_id: 'qt_1', state: 'pending', amount_fen: '9900', currency: 'CNY',
  payment: 'pending', fulfillment: 'pending', checkout_url: 'https://pay.example/qr', version: 1,
};

test('checkout renders frozen quote line items and submits a purchase', async () => {
  const calls: Array<{ quote_id: string; provider: string }> = [];
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async (input: { quote_id: string; provider: string }) => {
        calls.push(input);
        return { state: 'awaiting_payment', order, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' };
      },
      getOrder: async () => order,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '31' });
  let root: Root | undefined;
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  const text = document.body.textContent ?? '';
  assert.match(text, /¥99\.00/);                       // 冻结金额（AC1）
  assert.match(text, /订阅费|subscription_fee/);          // 行项目（AC1）
  assert.match(text, /advanced_models|高级模型/);         // 冻结权益（AC1）
  assert.match(text, /待付款/);                          // 产品状态（AC3）
  assert.equal(calls.length, 1);                        // 提交恰好一次购买
  assert.equal(calls[0]?.quote_id, 'qt_1');
  assert.equal(calls[0]?.provider, 'wechat');
  await act(async () => { root?.unmount(); });
});
