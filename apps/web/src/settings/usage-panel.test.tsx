import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

// packages/ui 旧栈 pulls in theme.css; node:test needs the same short-circuit as
// the other settings panel tests (CloudSettingsPanel.test.tsx).
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

import type { UsageRow } from '@weknora/contracts';
const { aggregateByModel, usageTotals, UsagePanel } = await import('./UsagePanel.tsx');

/* ocr1-024 回归用 harness：真挂载驱动原生 date input。安装在任何 test()
 * 注册之前（node:test 根用例在最后一个已注册用例结束且队列为空时会收尾，
 * 尾部安装 jsdom 会报 "asynchronous activity after the test ended"）。 */
const { JSDOM: UsageJSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const usageDom = new UsageJSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  window: usageDom.window,
  document: usageDom.window.document,
  HTMLElement: usageDom.window.HTMLElement,
  HTMLInputElement: usageDom.window.HTMLInputElement,
  Element: usageDom.window.Element,
  Node: usageDom.window.Node,
  Event: usageDom.window.Event,
  MouseEvent: usageDom.window.MouseEvent,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: usageDom.window.navigator });

const ReactUsage = await import('react');
const { act: actUsage } = await import('react');
const { createRoot: createUsageRoot } = await import('react-dom/client');
(globalThis as typeof globalThis & { React: unknown }).React = ReactUsage;

function row(overrides: Partial<UsageRow> & Pick<UsageRow, 'model'>): UsageRow {
  return {
    window_start: '2026-09-01T00:00:00Z',
    input_tokens: 0,
    output_tokens: 0,
    cache_read_tokens: 0,
    cache_write_tokens: 0,
    cost_microcredits: 0,
    ...overrides,
  };
}

test('aggregateByModel sums windows of the same model and merges cache read+write', () => {
  const items: UsageRow[] = [
    row({ model: 'gpt-test', window_start: '2026-09-01T00:00:00Z', input_tokens: 100, output_tokens: 40, cache_read_tokens: 10, cache_write_tokens: 5, cost_microcredits: 700 }),
    row({ model: 'gpt-test', window_start: '2026-09-02T00:00:00Z', input_tokens: 50, output_tokens: 60, cache_read_tokens: 20, cache_write_tokens: 15, cost_microcredits: 300 }),
    row({ model: 'bge-m3', window_start: '2026-09-01T00:00:00Z', input_tokens: 7, output_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 3, cost_microcredits: 1 }),
  ];
  assert.deepEqual(aggregateByModel(items), [
    { model: 'gpt-test', input: 150, output: 100, cache: 50, cost: 1000 },
    { model: 'bge-m3', input: 7, output: 0, cache: 3, cost: 1 },
  ]);
});

test('aggregateByModel keeps first-appearance model order', () => {
  const items: UsageRow[] = [
    row({ model: 'zeta' }),
    row({ model: 'alpha' }),
    row({ model: 'zeta', input_tokens: 1 }),
  ];
  assert.deepEqual(aggregateByModel(items).map((item) => item.model), ['zeta', 'alpha']);
  assert.equal(aggregateByModel(items)[0]!.input, 1);
});

test('aggregateByModel returns an empty list for empty input', () => {
  assert.deepEqual(aggregateByModel([]), []);
});

test('usageTotals sums every aggregate column into the footer row', () => {
  const rows = aggregateByModel([
    row({ model: 'gpt-test', input_tokens: 100, output_tokens: 40, cache_read_tokens: 10, cache_write_tokens: 5, cost_microcredits: 700 }),
    row({ model: 'bge-m3', input_tokens: 7, cache_write_tokens: 3, cost_microcredits: 1 }),
  ]);
  assert.deepEqual(usageTotals(rows), { model: '', input: 107, output: 40, cache: 18, cost: 701 });
});

test('usageTotals zeroes out when no models were used', () => {
  assert.deepEqual(usageTotals([]), { model: '', input: 0, output: 0, cache: 0, cost: 0 });
});

/* ocr1-024 回归：原生 <input type="date"> 的 onChange 收到的是合成事件，
 * 误写 TDesign 取值签名会把 '[object Object]' 写进日期草稿，applyRange 的
 * clampAnalyticsRange 解析失败回退默认窗口。真挂载驱动 date input → 应用 →
 * 断言 client.usage.my 收到所键日期。 */
test('native date inputs feed the applied usage range with event values, not "[object Object]" (ocr1-024)', async () => {
  const calls: Array<{ startTime?: string; endTime?: string }> = [];
  const client = {
    usage: { my: async (params: { startTime?: string; endTime?: string }) => { calls.push(params); return { items: [] }; } },
    commercial: { summary: async () => { throw new Error('no commercial'); } },
  };
  const container = document.createElement('div');
  document.body.append(container);
  const root = createUsageRoot(container);
  await actUsage(async () => { root.render(<UsagePanel client={client as never} />); });
  await actUsage(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
  assert.equal(calls.length, 1, 'mount loads once with the default range');
  assert.match(calls[0]?.startTime ?? '', /^\d{4}-\d{2}-\d{2}$/, 'the default range parses');

  const setDate = (input: HTMLInputElement, value: string) => {
    const setValue = Object.getOwnPropertyDescriptor(usageDom.window.HTMLInputElement.prototype, 'value')?.set;
    setValue?.call(input, value);
    input.dispatchEvent(new usageDom.window.Event('change', { bubbles: true }));
  };
  const inputs = Array.from(container.querySelectorAll('input[type="date"]')) as HTMLInputElement[];
  assert.equal(inputs.length, 2, 'the from/to date inputs render');
  await actUsage(async () => { setDate(inputs[0]!, '2026-09-01'); setDate(inputs[1]!, '2026-09-08'); });
  const apply = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '应用');
  assert.ok(apply, 'the apply button renders');
  await actUsage(async () => { apply!.click(); });
  assert.equal(calls.length, 2, 'apply triggers a reload');
  assert.equal(calls[1]?.startTime, '2026-09-01', 'the typed from-date reaches the request (UTC-day string contract)');
  assert.equal(calls[1]?.endTime, '2026-09-08', 'the typed to-date reaches the request (UTC-day string contract)');
  await actUsage(async () => { root.unmount(); });
});
