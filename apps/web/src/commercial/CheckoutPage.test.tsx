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
      purchaseStatus: async () => ({ state: 'absent' }),
      getOrder: async () => order,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '31' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
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
  assert.equal(calls[0]?.provider, 'alipay');           // (审查 H1) 默认渠道=支付宝（#82 主链）
  assert.match(text, /支付宝/);                          // 渠道选择器可见
  assert.match(text, /微信支付/);
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
      purchaseStatus: async () => ({ state: 'absent' }),
      getOrder: async (id: string) => { getOrderCalls.push(id); return order; },
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '41' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
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

// R1-24：POST /purchases 的平台故障走 503 错误信封（非 2xx → ApiError，
// 顶层 reason 令牌在 details 里）——页面必须按闭合令牌映射中文文案，而不是
// 把英文原始 message（"purchase temporarily unavailable"）直接展示。
test('a 503 purchase failure with a closed reason token maps to the Chinese message', async () => {
  const { ApiError } = await import('@weknora/api-client');
  const failure = new ApiError({
    status: 503,
    code: 'HTTP_503',
    message: 'purchase temporarily unavailable',
    details: { reason: 'unreachable' },
  });
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async () => { throw failure; },
      purchaseStatus: async () => ({ state: 'absent' }),
      getOrder: async () => order,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '43' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  const text = document.body.textContent ?? '';
  assert.match(text, /支付平台暂时不可达/);
  assert.doesNotMatch(text, /purchase temporarily unavailable/);
  await act(async () => { root?.unmount(); });
});

// R2-01：409/500 错误族不带 reason 令牌、只有 message 上的英文机器令牌——
// 页面按 message 令牌映射中文；未命中的服务端错误给统一中文兜底，英文机器
// 令牌不原文展示（spec L210 页面词汇稳定）。
test('a 409 purchase conflict maps its message token to the Chinese copy', async () => {
  const { ApiError } = await import('@weknora/api-client');
  const expired = new ApiError({ status: 409, code: 'HTTP_409', message: 'quote expired' });
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async () => { throw expired; },
      purchaseStatus: async () => ({ state: 'absent' }),
      getOrder: async () => order,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '45' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  const text = document.body.textContent ?? '';
  assert.match(text, /报价已过期/);
  assert.doesNotMatch(text, /quote expired/);
  await act(async () => { root?.unmount(); });
});

test('an unmapped server error falls back to the closed Chinese copy, never the raw token', async () => {
  const { ApiError } = await import('@weknora/api-client');
  const unmapped = new ApiError({ status: 409, code: 'HTTP_409', message: 'some_future_conflict_token' });
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async () => { throw unmapped; },
      purchaseStatus: async () => ({ state: 'absent' }),
      getOrder: async () => order,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '46' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  const text = document.body.textContent ?? '';
  assert.match(text, /购买未能创建，请稍后重试/);
  assert.doesNotMatch(text, /some_future_conflict_token/);
  await act(async () => { root?.unmount(); });
});

// R3-07：报价级冲突的重试必须走一次新报价（quoteRef 清空），不是对同一张
// 失效报价死循环。
test('a quote-level conflict retry re-cuts a fresh quote', async () => {
  const { ApiError } = await import('@weknora/api-client');
  const expired = new ApiError({ status: 409, code: 'HTTP_409', message: 'quote expired' });
  let quoteCalls = 0;
  let purchaseCalls = 0;
  const client = {
    commercial: {
      quote: async () => { quoteCalls += 1; return quote; },
      purchase: async () => {
        purchaseCalls += 1;
        if (purchaseCalls === 1) throw expired;
        return { state: 'awaiting_payment', order, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' };
      },
      purchaseStatus: async () => ({ state: 'absent' }),
      getOrder: async () => order,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '47' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  assert.match(document.body.textContent ?? '', /报价已过期/);
  assert.equal(quoteCalls, 1);
  // 点击重试：quoteRef 已被清空 → 重新调 quote（新报价），purchase 用新 quote。
  const retry = [...document.querySelectorAll('button')].find((b) => b.textContent?.includes('重试'));
  assert.ok(retry, 'the retry button must exist');
  await act(async () => { retry?.click(); });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  assert.equal(quoteCalls, 2, 'a quote-level conflict retry must re-cut a fresh quote');
  assert.equal(purchaseCalls, 2);
  await act(async () => { root?.unmount(); });
});

// R3-09：渠道失败（202 姿态——pending 订单无可用支付链接）不再是死胡同：
// 给出渠道异常提示与「重新发起支付」入口（新报价新订单）。
test('a link-less pending order offers a restart-checkout way out', async () => {
  const deadOrder = { ...order, checkout_url: '' };
  let quoteCalls = 0;
  let purchaseCalls = 0;
  const client = {
    commercial: {
      quote: async () => { quoteCalls += 1; return quote; },
      purchase: async () => {
        purchaseCalls += 1;
        if (purchaseCalls === 1) return { state: 'awaiting_payment', order: deadOrder, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' };
        return { state: 'awaiting_payment', order, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' };
      },
      purchaseStatus: async () => ({ state: 'awaiting_payment' }),
      getOrder: async () => deadOrder,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '48' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  const text = document.body.textContent ?? '';
  assert.match(text, /支付渠道异常/);
  assert.match(text, /重新发起支付/);
  const restart = [...document.querySelectorAll('button')].find((b) => b.textContent?.includes('重新发起支付'));
  assert.ok(restart, 'the restart button must exist');
  await act(async () => { restart?.click(); });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  assert.equal(quoteCalls, 2, 'restart must re-cut a quote');
  assert.equal(purchaseCalls, 2, 'restart must open a new order');
  const link = document.querySelector('a[href="https://pay.example/qr"]');
  assert.ok(link, 'the restarted checkout must render the payment link');
  await act(async () => { root?.unmount(); });
});

// R3-02：非服务端错误（网络层）收敛为中文兜底——浏览器英文技术串不渲染。
test('a network-layer error renders the closed Chinese copy', async () => {
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async () => { throw new TypeError('Failed to fetch'); },
      purchaseStatus: async () => ({ state: 'absent' }),
      getOrder: async () => order,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '49' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  const text = document.body.textContent ?? '';
  assert.match(text, /网络异常，请稍后重试/);
  assert.doesNotMatch(text, /Failed to fetch/);
  await act(async () => { root?.unmount(); });
});

// (审查 H1) 渠道选择器：默认支付宝；切到微信后提交体 provider=='wechat'
// （#81 既有形状由表驱动锁定）。
test('the channel selector defaults to alipay and submits the selected provider', async () => {
  const calls: Array<{ quote_id: string; provider: string }> = [];
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async (input: { quote_id: string; provider: string }) => {
        calls.push(input);
        return { state: 'awaiting_payment', order, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' };
      },
      purchaseStatus: async () => ({ state: 'awaiting_payment' }),
      getOrder: async () => order,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '71' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  assert.equal(calls[0]?.provider, 'alipay');
  // 切到微信并点「重新发起支付」：新订单的提交体 provider=='wechat'。
  const wechatRadio = document.querySelector<HTMLInputElement>('input[value="wechat"]');
  assert.ok(wechatRadio, 'the wechat radio must render');
  await act(async () => { wechatRadio?.click(); });
  const restart = [...document.querySelectorAll('button')].find((b) => b.textContent?.includes('改用微信支付重新发起支付'));
  assert.ok(restart, 'the switch-channel re-submit entry must appear once the radio changes');
  assert.ok(restart, 'a restart/retry button must exist');
  await act(async () => { restart?.click(); });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  assert.equal(calls[calls.length - 1]?.provider, 'wechat', 'the wechat selection must ride the next submit');
  await act(async () => { root?.unmount(); });
});

// (#82 AC1) 三态数据源是 purchase 投影：paid_awaiting_activation →「已付款，
// 权益处理中」；active →「权益已生效」。
test('the three-state face rides the purchase projection', async () => {
  for (const [purchaseState, want] of [
    ['paid_awaiting_activation', '已付款，权益处理中'],
    ['active', '权益已生效'],
    ['awaiting_payment', '待付款（权益未开通）'],
  ] as const) {
    const client = {
      commercial: {
        quote: async () => quote,
        purchase: async () => ({ state: purchaseState, order, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' }),
        purchaseStatus: async () => ({ state: purchaseState }),
        getOrder: async () => order,
      },
    };
    const { CheckoutPage } = await import('./CheckoutPage.tsx');
    const { createScopeController } = await import('@weknora/domain/scope');
    const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '72' });
    let root: Root | undefined;
    await act(async () => {
      root = createRoot(document.body.appendChild(document.createElement('div')));
      root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
    });
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
    assert.match(document.body.textContent ?? '', new RegExp(want));
    await act(async () => { root?.unmount(); });
  }
});

// R1-V12：已支付的订单不再显示「待付款（权益未开通）」标签（付款后回访/轮询
// 更新后不得误导重复支付）。
test('a paid order no longer shows the awaiting-payment label', async () => {
  const paidOrder = { ...order, payment: 'paid' as const };
  const client = {
    commercial: {
      quote: async () => quote,
      purchase: async () => ({ state: 'paid_awaiting_activation', order: paidOrder, plan_key: 'pro', plan_version: 1, amount_fen: '9900', currency: 'CNY' }),
      purchaseStatus: async () => ({ state: 'paid_awaiting_activation' }),
      getOrder: async () => paidOrder,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '51' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
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
      purchaseStatus: async () => ({ state: 'awaiting_payment' }),
      getOrder: async () => evilOrder,
    },
  };
  const { CheckoutPage } = await import('./CheckoutPage.tsx');
  const { createScopeController } = await import('@weknora/domain/scope');
  const scopeController = createScopeController({ origin: '', userId: 'user-1', tenantId: '61' });
  let root: Root | undefined;
  document.body.innerHTML = ''; // isolate each test's DOM face
  await act(async () => {
    root = createRoot(document.body.appendChild(document.createElement('div')));
    root.render(React.createElement(CheckoutPage, { client: client as never, scopeController, orderId: '' }));
  });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  assert.equal(document.querySelector('a[href="javascript:alert(1)"]'), null, 'dangerous scheme must not render');
  assert.equal(document.querySelector('a'), null, 'no payment link at all for an unsafe checkout_url');
  await act(async () => { root?.unmount(); });
});
