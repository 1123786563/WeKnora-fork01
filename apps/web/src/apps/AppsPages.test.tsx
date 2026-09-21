import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import React from 'react';
import { act } from 'react';

const hooks = createRequire(import.meta.url)('node:module') as typeof import('node:module') & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
/* tdesign 运行时依赖的 DOM 全局必须在模块加载前就位：_util/listener.js 的
 * 事件绑定函数是模块级 IIFE，导入时若 document.addEventListener 缺席会
 * 固化到 IE 时代的 attachEvent 分支（jsdom 无此 API）——pilot/kb-list 同款
 * 顶层 JSDOM + 全局补齐。 */
const harnessDom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/apps' });
Object.assign(globalThis, {
  React,
  IS_REACT_ACT_ENVIRONMENT: true,
  window: harnessDom.window,
  document: harnessDom.window.document,
  HTMLElement: harnessDom.window.HTMLElement,
  HTMLInputElement: harnessDom.window.HTMLInputElement,
  HTMLTextAreaElement: harnessDom.window.HTMLTextAreaElement,
  HTMLSelectElement: harnessDom.window.HTMLSelectElement,
  Element: harnessDom.window.Element,
  Node: harnessDom.window.Node,
  SVGElement: harnessDom.window.SVGElement,
  DocumentFragment: harnessDom.window.DocumentFragment,
  Event: harnessDom.window.Event,
  KeyboardEvent: harnessDom.window.KeyboardEvent,
  MouseEvent: harnessDom.window.MouseEvent,
  MutationObserver: harnessDom.window.MutationObserver,
  getComputedStyle: harnessDom.window.getComputedStyle?.bind(harnessDom.window),
  requestAnimationFrame: harnessDom.window.requestAnimationFrame?.bind(harnessDom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: harnessDom.window.cancelAnimationFrame?.bind(harnessDom.window) ?? clearTimeout,
});
const { AppsPage } = await import('./AppsPages.tsx');

/* Vue ActionView parity: the approval card renders the FROZEN risk from the
   persisted snapshot (apps.risk.* label verbatim); an empty payload degrades
   to an honest em dash with the riskUnknownHint — never a guess. */
function renderActionPage(actionDto: unknown, props: Record<string, unknown> = {}): Promise<{ container: HTMLElement; cleanup: () => Promise<void> }> {
  return (async () => {
    const dom = harnessDom;
    const { createRoot } = await import('react-dom/client');
    const container = dom.window.document.createElement('div');
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);
    const client = { request: async () => actionDto } as never;
    await act(async () => {
      root.render(React.createElement(AppsPage, { client, mode: 'action', id: 'action-1', ...props }));
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    await act(async () => { await new Promise((resolve) => setImmediate(resolve)); });
    return {
      container,
      cleanup: async () => {
        await act(async () => { root.unmount(); });
        document.body.replaceChildren();
      },
    };
  })();
}

async function renderCatalogPage(): Promise<{ container: HTMLElement; cleanup: () => Promise<void> }> {
  const dom = harnessDom;
  const { createRoot } = await import('react-dom/client');
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  const client = { request: async () => [] } as never;
  await act(async () => {
    root.render(React.createElement(AppsPage, { client, mode: 'catalog' }));
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return {
    container,
    cleanup: async () => {
      await act(async () => { root.unmount(); });
      document.body.replaceChildren();
    },
  };
}

test('catalog tables use the Vue unboxed section layout and centered empty states', async () => {
  const { container, cleanup } = await renderCatalogPage();
  try {
    assert.equal(container.querySelectorAll('section.rounded-card').length, 0, 'catalog and installed tables are not wrapped in cards');
    assert.ok(container.querySelector('.apps-view__header button svg'), 'refresh control includes the Vue refresh icon (tdesign sprite use)');
    // tdesign t-table empty：双表各渲染一个 .t-table__empty 占位块。
    const emptyBlocks = Array.from(container.querySelectorAll('.t-table__empty'));
    assert.equal(emptyBlocks.length, 2, 'catalog + installed tables each render the t-table empty block');
    for (const block of emptyBlocks) {
      assert.ok(block.textContent, 'empty block carries its description text');
    }
  } finally {
    await cleanup();
  }
});

test('renders the frozen delete risk verbatim like the Vue ActionView', async () => {
  const { container, cleanup } = await renderActionPage({ action: { risk: 'delete', state: 'awaiting_approval', content: '{}' } });
  try {
    const riskLabel = Array.from(container.querySelectorAll('dt')).find((element) => element.textContent === '风险');
    assert.ok(riskLabel, 'renders the risk field');
    assert.equal(riskLabel.nextElementSibling?.textContent, '删除');
  } finally {
    await cleanup();
  }
});

test('renders an unknown action risk as an em dash with the honest hint', async () => {
  const { container, cleanup } = await renderActionPage({ action: { state: 'awaiting_approval', content: '{}' } });
  try {
    const riskLabel = Array.from(container.querySelectorAll('dt')).find((element) => element.textContent === '风险');
    assert.ok(riskLabel, 'renders the risk field');
    assert.equal(riskLabel.nextElementSibling?.getAttribute('title'), '当前接口未返回该字段；风险以服务端冻结数据为准');
    assert.equal(riskLabel.nextElementSibling?.textContent, '—');
  } finally {
    await cleanup();
  }
});

test('approve controls follow actionControls: awaiting_approval + permission shows 批准', async () => {
  const { container, cleanup } = await renderActionPage({ action: { risk: 'read', state: 'awaiting_approval', content: '{"a":1}' } }, { role: 'owner' });
  try {
    const approve = Array.from(container.querySelectorAll('button')).find((element) => element.textContent === '批准');
    assert.ok(approve, 'approve button present for awaiting_approval');
    const execute = Array.from(container.querySelectorAll('button')).find((element) => element.textContent === '执行');
    assert.equal(execute, undefined, 'no execute button before approval');
  } finally {
    await cleanup();
  }
});

test('a non-admin viewer sees the member hint and no approval controls', async () => {
  const { container, cleanup } = await renderActionPage({ action: { risk: 'read', state: 'awaiting_approval', content: '{}' } });
  try {
    assert.equal(Array.from(container.querySelectorAll('button')).find((element) => element.textContent === '批准'), undefined);
    assert.match(container.textContent ?? '', /当前角色无法审批或执行动作/);
  } finally {
    await cleanup();
  }
});
