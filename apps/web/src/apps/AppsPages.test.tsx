import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import React from 'react';
import { act } from 'react';

const hooks = createRequire(import.meta.url)('node:module') as typeof import('node:module') & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
Object.assign(globalThis, { React, IS_REACT_ACT_ENVIRONMENT: true });
const { AppsPage } = await import('./AppsPages.tsx');

/* Vue ActionView parity: the approval card renders the FROZEN risk from the
   persisted snapshot (apps.risk.* label verbatim); an empty payload degrades
   to an honest em dash with the riskUnknownHint — never a guess. */
function renderActionPage(actionDto: unknown, props: Record<string, unknown> = {}): Promise<{ container: HTMLElement; cleanup: () => Promise<void> }> {
  return (async () => {
    const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/apps/actions/action-1' });
    const previousWindow = globalThis.window;
    const previousDocument = globalThis.document;
    Object.assign(globalThis, { window: dom.window, document: dom.window.document });
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
        Object.assign(globalThis, { window: previousWindow, document: previousDocument });
        dom.window.close();
      },
    };
  })();
}

async function renderCatalogPage(): Promise<{ container: HTMLElement; cleanup: () => Promise<void> }> {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/apps' });
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  Object.assign(globalThis, { window: dom.window, document: dom.window.document });
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
      Object.assign(globalThis, { window: previousWindow, document: previousDocument });
      dom.window.close();
    },
  };
}

test('catalog tables use the Vue unboxed section layout and centered empty states', async () => {
  const { container, cleanup } = await renderCatalogPage();
  try {
    assert.equal(container.querySelectorAll('section.rounded-card').length, 0, 'catalog and installed tables are not wrapped in cards');
    assert.ok(container.querySelector('header button svg[aria-hidden="true"]'), 'refresh control includes the Vue refresh icon');
    const emptyCells = Array.from(container.querySelectorAll('td'));
    assert.equal(emptyCells.length, 2);
    for (const cell of emptyCells) {
      assert.ok(cell.className.includes('text-center'), 'Vue table empty state is centered');
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
