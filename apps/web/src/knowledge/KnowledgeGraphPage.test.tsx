import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import * as React from 'react';
import { act } from 'react';
import test, { afterEach } from 'node:test';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledgeBase/kb-1/graph' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  SVGElement: dom.window.SVGElement,
  Event: dom.window.Event,
  PointerEvent: dom.window.PointerEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'en-US' });

const { createRoot } = await import('react-dom/client');
const { KnowledgeGraphPage } = await import('./KnowledgeGraphPage.tsx');

let root: Root | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

function graphClient(): WeKnoraClient {
  return {
    wiki: {
      graph: async () => ({
        nodes: [
          { slug: 'start', title: 'Start', page_type: 'summary', link_count: 2, familiar: true },
          { slug: 'next', title: 'Next', page_type: 'entity', link_count: 1 },
          { slug: 'third', title: 'Third', page_type: 'concept', link_count: 1 },
        ],
        edges: [
          { source: 'start', target: 'next' },
          { source: 'next', target: 'start' },
          { source: 'third', target: 'next' },
        ],
        meta: { mode: 'overview', total: 3, returned: 3, truncated: false },
      }),
      list: async () => ({ pages: [], total: 0, page: 1, page_size: 20, total_pages: 1 }),
      get: async (knowledgeBaseId: string, slug: string) => ({ title: slug, summary: '', content: '# Details\n\n**Markdown**', version: 1, slug, knowledgeBaseId }),
    },
  } as unknown as WeKnoraClient;
}

async function mount() {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => {
    root?.render(<KnowledgeGraphPage client={graphClient()} knowledgeBaseId="kb-1" />);
  });
  await act(async () => {});
  return container;
}

test('graph arrows follow the Vue toggle and reciprocal-edge rendering contract', async () => {
  const container = await mount();
  const line = container.querySelector('svg line');
  assert.ok(line);
  assert.equal(line.getAttribute('marker-end'), 'url(#wk-graph-arrow-end)');
  assert.equal(line.getAttribute('marker-start'), 'url(#wk-graph-arrow-start)');
  assert.equal(container.querySelectorAll('svg line').length, 2, 'reciprocal links share one line like Vue');

  const toggle = container.querySelector<HTMLButtonElement>('button[aria-pressed]');
  assert.ok(toggle);
  assert.equal(toggle.getAttribute('aria-pressed'), 'true');
  await act(async () => toggle.click());
  assert.equal(toggle.getAttribute('aria-pressed'), 'false');
  assert.equal(line.getAttribute('marker-end'), null);
  assert.equal(line.getAttribute('marker-start'), null);
  await act(async () => toggle.click());
  assert.equal(toggle.getAttribute('aria-pressed'), 'true');
  assert.equal(line.getAttribute('marker-end'), 'url(#wk-graph-arrow-end)');
  assert.equal(line.getAttribute('marker-start'), 'url(#wk-graph-arrow-start)');
});

test('graph help lists every canvas gesture documented by Vue', async () => {
  const container = await mount();
  const help = container.querySelector('details');
  assert.ok(help);

  const heading = help.querySelector('dl > div:not(.grid)');
  assert.equal(heading?.textContent?.trim(), '画布操作');
  const rows = [...help.querySelectorAll('dl div.grid')].map((row) => row.textContent?.trim());
  assert.deepEqual(rows, [
    '单击打开节点详情',
    '双击以该节点为中心聚焦',
    'Shift + 单击叠加该节点邻居到画布',
    '悬浮 → ⊕同 Shift + 单击',
    '拖拽节点手动调整节点位置',
    '拖拽空白平移画布',
    '滚轮缩放画布',
  ]);
});

test('graph keeps Vue canvas overlays and familiar-node ring semantics', async () => {
  const container = await mount();
  assert.ok(container.querySelector('[data-testid="knowledge-graph-surface"]'));
  assert.ok(container.querySelector('[data-testid="knowledge-graph-legend"]'));
  const familiarRing = container.querySelector('svg .wk-graph-familiar-ring');
  assert.ok(familiarRing);
  assert.match(familiarRing?.getAttribute('class') ?? '', /stroke:#0052d9/);
  assert.ok(Number(familiarRing?.getAttribute('r')) > 10);
});

test('graph drawer renders page Markdown as reader content', async () => {
  const container = await mount();
  const node = container.querySelector<SVGGElement>('svg g[role="button"]');
  assert.ok(node);
  await act(async () => node.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })));
  await act(async () => {});
  assert.ok(container.querySelector('[data-testid="knowledge-graph-reader"] h1'));
  assert.equal(container.querySelector('[data-testid="knowledge-graph-reader"] strong')?.textContent, 'Markdown');
  assert.equal(container.querySelector('[data-testid="knowledge-graph-reader"] pre'), null);
});
