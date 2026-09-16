import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import * as React from 'react';
import { act } from 'react';
import test, { afterEach } from 'node:test';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

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
const { graphNodeRadius, layoutGraphNodes } = await import('./graph.ts');

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
      list: async () => ({ pages: [{ title: 'Manual Page', slug: 'manual-page' }], total: 1, page: 1, page_size: 20, total_pages: 1 }),
      get: async (knowledgeBaseId: string, slug: string) => ({ title: slug, summary: '', content: '# Details\n\n**Markdown**', version: 1, slug, knowledgeBaseId }),
    },
    // Header chrome inputs (Vue KnowledgeBase.vue: kbInfo, auth/me for the
    // manage gate, KBSwitcherDropdown list, parser engines for KBInfoPopover).
    knowledgeBases: {
      settings: {
        get: async () => ({
          id: 'kb-1',
          name: 'Wiki图谱fixture',
          type: 'document',
          description: '图谱页头测试知识库',
          created_at: '2026-01-02T03:04:05Z',
          indexing_strategy: { wiki_enabled: true, graph_enabled: true },
          chunking_config: {},
        }),
        parserEngines: async () => ({ data: [{ Name: 'builtin', FileTypes: ['pdf', 'md'], Available: true }] }),
      },
      list: async () => [{ id: 'kb-1', name: 'Wiki图谱fixture' }],
    },
    auth: {
      me: async () => ({ user: { id: 'u-1', role: 'admin' } }),
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
  // Scope to the canvas svg's transform group: the legend arrow-toggle button
  // (also inside the surface) carries its own inline svg icon with a line.
  const graphGroup = container.querySelector('[data-testid="knowledge-graph-surface"] svg > g');
  assert.ok(graphGroup);
  const line = graphGroup.querySelector('line');
  assert.ok(line);
  assert.equal(line.getAttribute('marker-end'), 'url(#wk-graph-arrow-end)');
  assert.equal(line.getAttribute('marker-start'), 'url(#wk-graph-arrow-start)');
  assert.equal(graphGroup.querySelectorAll('line').length, 2, 'reciprocal links share one line like Vue');

  // Marker geometry mirrors the Vue WikiBrowser SVG defs (arrow-end / arrow-start).
  const markerEnd = container.querySelector('marker#wk-graph-arrow-end');
  assert.ok(markerEnd);
  assert.equal(markerEnd.getAttribute('viewBox'), '0 0 10 6');
  assert.equal(markerEnd.getAttribute('refX'), '10');
  assert.equal(markerEnd.getAttribute('refY'), '3');
  assert.equal(markerEnd.getAttribute('markerWidth'), '8');
  assert.equal(markerEnd.getAttribute('markerHeight'), '6');
  assert.equal(markerEnd.getAttribute('orient'), 'auto');
  assert.equal(markerEnd.querySelector('path')?.getAttribute('d'), 'M0,0 L10,3 L0,6 L2,3 Z');
  assert.equal(markerEnd.querySelector('path')?.getAttribute('fill'), '#c0c4cc');
  const markerStart = container.querySelector('marker#wk-graph-arrow-start');
  assert.ok(markerStart);
  assert.equal(markerStart.getAttribute('refX'), '0');
  assert.equal(markerStart.querySelector('path')?.getAttribute('d'), 'M10,0 L0,3 L10,6 L8,3 Z');
  assert.equal(markerStart.querySelector('path')?.getAttribute('fill'), '#c0c4cc');

  // Edges stop at the node circle boundary (+4px) like Vue setEdgePositions,
  // otherwise the ~9.6px arrows are painted over by the node circles.
  const [start, next] = layoutGraphNodes([
    { slug: 'start', title: 'Start', page_type: 'summary', link_count: 2, familiar: true },
    { slug: 'next', title: 'Next', page_type: 'entity', link_count: 1 },
    { slug: 'third', title: 'Third', page_type: 'concept', link_count: 1 },
  ], 760, 420);
  const dx = next!.x - start!.x;
  const dy = next!.y - start!.y;
  const dist = Math.hypot(dx, dy);
  const expectedX1 = start!.x + (dx / dist) * (graphNodeRadius(2) + 4);
  const expectedY1 = start!.y + (dy / dist) * (graphNodeRadius(2) + 4);
  const expectedX2 = next!.x - (dx / dist) * (graphNodeRadius(1) + 4);
  const expectedY2 = next!.y - (dy / dist) * (graphNodeRadius(1) + 4);
  assert.ok(Math.abs(Number(line.getAttribute('x1')) - expectedX1) < 1e-6, 'source end shortened by node radius + 4');
  assert.ok(Math.abs(Number(line.getAttribute('y1')) - expectedY1) < 1e-6);
  assert.ok(Math.abs(Number(line.getAttribute('x2')) - expectedX2) < 1e-6, 'target end shortened by node radius + 4');
  assert.ok(Math.abs(Number(line.getAttribute('y2')) - expectedY2) < 1e-6);
  assert.equal(line.getAttribute('class')?.includes('stroke-opacity:0.4'), true);

  const toggle = container.querySelector<HTMLButtonElement>('button[aria-pressed]');
  assert.ok(toggle);
  assert.equal(toggle.getAttribute('aria-pressed'), 'true');
  // Vue icon semantics: browse-off (slashed eye) while arrows are shown.
  assert.ok(toggle.querySelector('svg line'), 'browse-off slash visible while arrows are on');
  await act(async () => toggle.click());
  assert.equal(toggle.getAttribute('aria-pressed'), 'false');
  assert.equal(line.getAttribute('marker-end'), null);
  assert.equal(line.getAttribute('marker-start'), null);
  assert.equal(toggle.querySelector('svg line'), null, 'browse icon without slash once arrows are hidden');
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

test('graph search renders the Vue t-select shell (search prefix + chevron suffix)', async () => {
  // Placeholder/aria-label copy asserts need a pinned locale (default is zh-CN).
  window.localStorage.setItem('locale', 'en-US');
  const container = await mount();
  const input = container.querySelector<HTMLInputElement>('input[role="combobox"]');
  assert.ok(input, 'search control is a combobox like the Vue t-select');
  assert.equal(input.getAttribute('aria-expanded'), 'false');
  assert.equal(input.getAttribute('aria-controls'), 'wk-graph-search-results');
  assert.equal(input.getAttribute('aria-autocomplete'), 'list');
  assert.equal(input.getAttribute('placeholder'), 'Search wiki pages...');
  assert.equal(input.getAttribute('aria-label'), 'Search');
  // Prefix search icon, suffix chevron button — the select-form suffix arrow.
  assert.equal(input.previousElementSibling?.tagName, 'svg', 'search prefix icon sits inside the control shell');
  const chevron = input.nextElementSibling as HTMLButtonElement | null;
  assert.ok(chevron?.matches('button[data-testid="graph-search-chevron"]'), 'chevron suffix toggle present');
  assert.ok(chevron?.querySelector('svg path[d="M1 1l4 4 4-4"]'), 'suffix arrow is a chevron-down');
  // Popup starts collapsed and the old always-visible label text is gone.
  assert.equal(container.querySelector('#wk-graph-search-results'), null);
  const rowText = (input.closest('div.flex.items-center')?.textContent ?? '').trim();
  assert.equal(rowText, '', 'placeholder-only select, no visible label text');
});

test('graph search expands like the Vue select: empty keyword shows the overview snapshot', async () => {
  const container = await mount();
  const input = container.querySelector<HTMLInputElement>('input[role="combobox"]');
  assert.ok(input);
  const chevron = input.nextElementSibling as HTMLButtonElement;
  await act(async () => chevron.click());
  assert.equal(input.getAttribute('aria-expanded'), 'true');
  const listbox = container.querySelector('#wk-graph-search-results');
  assert.ok(listbox);
  assert.equal(listbox.getAttribute('role'), 'listbox');
  // Vue graphSearchEffectiveOptions falls back to the link_count-ranked snapshot.
  const titles = [...listbox.querySelectorAll('button')].map((option) => option.textContent);
  assert.deepEqual(titles, ['Start', 'Next', 'Third']);
  await act(async () => chevron.click());
  assert.equal(input.getAttribute('aria-expanded'), 'false');
  assert.equal(container.querySelector('#wk-graph-search-results'), null);
  // Focus reopens; Escape collapses; outside pointerdown collapses too.
  await act(async () => input.focus());
  assert.equal(input.getAttribute('aria-expanded'), 'true');
  await act(async () => input.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })));
  assert.equal(input.getAttribute('aria-expanded'), 'false');
  await act(async () => input.focus());
  await act(async () => document.body.dispatchEvent(new dom.window.PointerEvent('pointerdown', { bubbles: true })));
  assert.equal(input.getAttribute('aria-expanded'), 'false');
});

test('graph search keyboard mirrors the Vue select: arrows highlight, Enter jumps', async () => {
  const container = await mount();
  const input = container.querySelector<HTMLInputElement>('input[role="combobox"]');
  assert.ok(input);
  const chevron = input.nextElementSibling as HTMLButtonElement;
  await act(async () => chevron.click());
  await act(async () => input.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })));
  assert.equal(input.getAttribute('aria-activedescendant'), 'wk-graph-search-option-0');
  assert.equal(container.querySelector('#wk-graph-search-results li[aria-selected="true"]')?.id, 'wk-graph-search-option-0');
  await act(async () => input.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })));
  await act(async () => {});
  // Committing pivots to the ego view: status flips to loading and the search
  // overlay remounts, so re-query the live combobox instead of the stale node.
  const liveInput = container.querySelector<HTMLInputElement>('input[role="combobox"]');
  assert.ok(liveInput);
  assert.equal(liveInput.getAttribute('aria-expanded'), 'false', 'popup collapses after committing');
  assert.equal(liveInput.value, '', 'keyword cleared like Vue graphSearchValue reset');
  // Selecting jumps to the ego view + opens the drawer, like Vue handleGraphSearchSelect.
  assert.ok(container.querySelector('[data-testid="knowledge-graph-reader"]'));
});

test('graph search falls back to remote wiki results while typing', async () => {
  const container = await mount();
  const input = container.querySelector<HTMLInputElement>('input[role="combobox"]');
  assert.ok(input);
  await act(async () => input.focus());
  await act(async () => {
    // React's value tracker ignores plain `.value` writes — go through the
    // native setter so the dispatched input event triggers onChange.
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!;
    setValue.call(input, 'man');
    input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 400)); });
  const listbox = container.querySelector('#wk-graph-search-results');
  assert.ok(listbox);
  const titles = [...listbox.querySelectorAll('button')].map((option) => option.textContent);
  assert.deepEqual(titles, ['Manual Page']);
});

test('graph header mirrors the Vue KB page chrome: breadcrumb, tab row, info/settings, subtitle', async () => {
  const container = await mount();
  const breadcrumb = container.querySelector('h2.document-breadcrumb');
  assert.ok(breadcrumb, 'breadcrumb renders inside the KB-embedded title row');

  // Crumb 1: 知识库 list link (menu.knowledgeBase), crumb 2: KB name.
  const crumbs = [...breadcrumb.querySelectorAll('.breadcrumb-link')].map((crumb) => crumb.textContent?.trim());
  assert.equal(crumbs[0], '知识库');
  assert.match(breadcrumb.textContent ?? '', /Wiki图谱fixture/);
  // The React-invented standalone "知识图谱" page title is gone.
  assert.equal(container.querySelector('h1'), null);

  // Crumb 3: the 文档 / Wiki / 图谱 tab row (Vue isWiki breadcrumb-tabs);
  // the active graph tab is brand-green, aria-current, and carries the
  // tabGraphTip concept-clarification tooltip. Hrefs reuse the canonical
  // KB route form (/knowledgeBase/<id>?tab=…) — the same URLs the documents
  // page nav links to; no new routes.
  const tabs = [...breadcrumb.querySelectorAll('a.breadcrumb-tab')].map((tab) => ({ label: tab.textContent?.trim(), href: tab.getAttribute('href') }));
  assert.deepEqual(tabs, [
    { label: '文档', href: '/knowledgeBase/kb-1' },
    { label: 'Wiki', href: '/knowledgeBase/kb-1?tab=wiki' },
    { label: '图谱', href: '/knowledgeBase/kb-1?tab=graph' },
  ]);
  const separators = [...breadcrumb.querySelectorAll('.breadcrumb-tab-sep')].map((sep) => sep.textContent?.trim());
  assert.deepEqual(separators, ['/', '/']);
  const active = breadcrumb.querySelector('a.breadcrumb-tab.is-active');
  assert.ok(active);
  assert.equal(active.getAttribute('aria-current'), 'page');
  assert.equal(active.textContent?.trim(), '图谱');
  assert.match(active.className, /text-\[var\(--wk-brand,#07c05f\)\]/, 'active tab uses the brand green highlight');
  assert.match(active.getAttribute('title') ?? '', /引用关系图/);

  // Title-row actions: ⓘ info popover + ⚙ settings gear (kb-title-actions).
  assert.ok(container.querySelector('.kb-title-actions .kb-info-button'), 'ⓘ info button present');
  assert.ok(container.querySelector('.kb-title-actions .kb-settings-button'), '⚙ settings button present');
  assert.equal(container.querySelector('.kb-settings-button')?.getAttribute('aria-label'), '设置');

  // Subtitle keeps the document upload copy under the graph tab (Vue renders
  // document-subtitle unconditionally across tabs).
  const subtitle = container.querySelector('.document-subtitle');
  assert.equal(subtitle?.textContent, '支持点击或拖拽上传，多格式文档自动解析并智能分块，快速构建可检索的知识库');
});

test('graph header collapses to the plain 文档 crumb when wiki is disabled (Vue non-wiki branch)', async () => {
  const client = graphClient() as WeKnoraClient & {
    knowledgeBases: { settings: { get: () => Promise<Record<string, unknown>> } };
  };
  client.knowledgeBases.settings.get = async () => ({
    id: 'kb-1',
    name: 'Plain库',
    type: 'document',
    indexing_strategy: { wiki_enabled: false, graph_enabled: false },
    chunking_config: {},
  });
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => {
    root?.render(<KnowledgeGraphPage client={client} knowledgeBaseId="kb-1" />);
  });
  await act(async () => {});
  const breadcrumb = container.querySelector('h2.document-breadcrumb');
  assert.ok(breadcrumb);
  assert.equal(breadcrumb.querySelectorAll('a.breadcrumb-tab').length, 0, 'no tab row for documents-only KBs');
  assert.match(breadcrumb.querySelector('.breadcrumb-current')?.textContent ?? '', /文档/);
});
