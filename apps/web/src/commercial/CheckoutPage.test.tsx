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
  // 支付跳转链接（审查 F2：渠道请求创建后用户必须有支付入口）。
  const payLink = document.querySelector('a[href="https://pay.example/qr"]');
  assert.ok(payLink, 'checkout must render the payment link from checkout_url');
  assert.match(payLink?.textContent ?? '', /前往支付|支付/);
  await act(async () => { root?.unmount(); });
});

// R1-V01：purchase.order 缺席（后端故障/未配置的闭合应答）时，页面不得发起
// 空 ID 的 getOrder 请求，而应渲染闭合 reason 的失败文案。
test('purchase without an order shows the closed failure message and never queries an empty order id', async () => {
  const getOrderCalls: string[] = [];
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async () => ({ state: 'absent', reason: 'unreachable' }),
      getOrder: async (id: string) => { getOrderCalls.push(id); return order; },
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '41' });
  let root: Root | undefined;
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  assert.deepEqual(getOrderCalls, [], 'no getOrder may be issued when the purchase carries no order');
  assert.match(document.body.textContent ?? '', /支付平台暂时不可达/);
  assert.match(document.body.textContent ?? '', /重试/);
  await act(async () => { root?.unmount(); });
});

// R1-V12：已支付的订单不再显示「待付款（权益未开通）」标签（付款后回访/轮询
// 更新后不得误导重复支付）。
test('a paid order no longer shows the awaiting-payment label', async () => {
  const paidOrder = { ...order, payment: 'paid' as const };
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async () => ({ state: 'awaiting_payment', order: paidOrder, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' }),
      getOrder: async () => paidOrder,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '51' });
  let root: Root | undefined;
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  const text = document.body.textContent ?? '';
  assert.doesNotMatch(text, /待付款（权益未开通）/, 'paid order must not show the awaiting-payment label');
  assert.match(text, /已付款，权益处理中/);
  await act(async () => { root?.unmount(); });
});

// R1-V13：危险 scheme 的 checkout_url 一律不渲染为链接。
test('a javascript-scheme checkout_url is never rendered as a link', async () => {
  const evilOrder = { ...order, checkout_url: 'javascript:alert(1)' };
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async () => ({ state: 'awaiting_payment', order: evilOrder, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' }),
      getOrder: async () => evilOrder,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '61' });
  let root: Root | undefined;
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  assert.equal(document.querySelector('a[href="javascript:alert(1)"]'), null, 'dangerous scheme must not render');
  assert.equal(document.querySelector('a'), null, 'no payment link at all for an unsafe checkout_url');
  await act(async () => { root?.unmount(); });
});
