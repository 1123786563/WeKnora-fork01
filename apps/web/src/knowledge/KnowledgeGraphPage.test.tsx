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
  SVGSVGElement: dom.window.SVGSVGElement,
  SVGGElement: dom.window.SVGGElement,
  Event: dom.window.Event,
  // openContextualGuide dispatches `new CustomEvent(...)`; without this pin the
  // Node-global CustomEvent wins and jsdom rejects the instance on dispatchEvent.
  CustomEvent: dom.window.CustomEvent,
  PointerEvent: dom.window.PointerEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'en-US' });

const { createRoot } = await import('react-dom/client');
const { KnowledgeGraphPage } = await import('./KnowledgeGraphPage.tsx');
const { graphNodeRadius, layoutGraphNodes } = await import('./graph.ts');
const { CONTEXTUAL_GUIDE_PENDING_KEY, OPEN_CONTEXTUAL_GUIDE_EVENT } = await import('../../../../packages/views/src/guides/contextual-guides.ts');

let root: Root | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
  window.sessionStorage.clear();
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

async function mount(client: WeKnoraClient = graphClient()) {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => {
    root?.render(<KnowledgeGraphPage client={client} knowledgeBaseId="kb-1" />);
  });
  await act(async () => {});
  return container;
}

// Real-browser activation path for canvas nodes: pointerdown registers the
// drag gesture and a stationary pointerup is the tap (endGraphGesture). A
// synthetic MouseEvent('click') bypasses the gesture system entirely — which
// is exactly how the old capture-on-pointerdown bug stayed invisible here.
function tapNode(node: SVGGElement, pointerId = 1) {
  node.dispatchEvent(new dom.window.PointerEvent('pointerdown', { bubbles: true, pointerId, button: 0, clientX: 10, clientY: 10 }));
  node.dispatchEvent(new dom.window.PointerEvent('pointerup', { bubbles: true, pointerId, button: 0, clientX: 10, clientY: 10 }));
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
  await act(async () => tapNode(node));
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
  const gear = container.querySelector<HTMLButtonElement>('.kb-settings-button');
  assert.equal(gear?.getAttribute('aria-label'), '设置');
  assert.equal(gear?.getAttribute('title'), '设置', 'Vue wraps the gear in t-tooltip knowledgeBase.settings');

  // ⚙ opens the in-place KB settings overlay (Vue uiStore.openKBSettings,
  // KnowledgeBase.vue:2388): a Dialog hosting the settled KB settings surface
  // instead of navigating to the /settings route.
  const pathBeforeGear = window.location.pathname;
  await act(async () => gear!.click());
  const dialog = document.querySelector('.wk-dialog');
  assert.ok(dialog, '⚙ opens the settings Dialog overlay');
  assert.equal(dialog?.getAttribute('role'), 'dialog');
  assert.equal(dialog?.querySelector('.wk-dialog-header h2')?.textContent, '设置');
  assert.ok(dialog?.querySelector('[aria-label^="Knowledge settings for"]'), 'Dialog hosts the KB settings surface');
  assert.equal(window.location.pathname, pathBeforeGear, 'the gear opens the overlay in place — no navigation');
  // Closing returns to the graph page.
  await act(async () => (dialog!.querySelector('.wk-dialog-close') as HTMLButtonElement).click());
  assert.equal(document.querySelector('.wk-dialog'), null, 'Dialog closes back into the graph page');

  // Subtitle keeps the document upload copy under the graph tab (Vue renders
  // document-subtitle unconditionally across tabs).
  const subtitle = container.querySelector('.document-subtitle');
  assert.equal(subtitle?.textContent, '支持点击或拖拽上传，多格式文档自动解析并智能分块，快速构建可检索的知识库');
});

test('graph header collapses to the plain 文档 crumb when wiki is disabled (Vue non-wiki branch)', async () => {
  // R432: the wiki gate is strict — a graph-enabled KB with the wiki off gets
  // no tab row either (Vue gates the whole row on isWiki, KnowledgeBase.vue:89,
  // 2359-2381; the graph view lives inside the wiki surface).
  const client = graphClient() as WeKnoraClient & {
    knowledgeBases: { settings: { get: () => Promise<Record<string, unknown>> } };
  };
  client.knowledgeBases.settings.get = async () => ({
    id: 'kb-1',
    name: 'Plain库',
    type: 'document',
    indexing_strategy: { wiki_enabled: false, graph_enabled: true },
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
  assert.equal(breadcrumb.querySelectorAll('a.breadcrumb-tab').length, 0, 'no tab row when the wiki is off, even with graph extraction on');
  assert.match(breadcrumb.querySelector('.breadcrumb-current')?.textContent ?? '', /文档/);
  // Vue keeps the ?tab=… URL readable only for wiki KBs (KnowledgeBase.vue:2412
  // gates .wiki-main-area on isWiki and renders the documents branch otherwise),
  // so the React deep link falls back to the canonical documents URL.
  assert.equal(window.location.pathname, '/knowledgeBase/kb-1', 'non-wiki ?tab=graph deep link falls back to the documents URL');
});

// --- R432 W2: search box ↔ canvas decoupling + debounce parity (Vue
// WikiBrowser.vue L4680-4712) ---

function typingHelper() {
  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!;
  return (input: HTMLInputElement, value: string) => {
    setValue.call(input, value);
    input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  };
}

function countingClient() {
  const client = graphClient();
  let listCalls = 0;
  const inner = client.wiki.list.bind(client.wiki);
  client.wiki.list = (async (...args: Parameters<typeof inner>) => {
    listCalls += 1;
    return inner(...args);
  }) as typeof client.wiki.list;
  return { client, calls: () => listCalls };
}

async function mountWith(client: WeKnoraClient) {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => {
    root?.render(<KnowledgeGraphPage client={client} knowledgeBaseId="kb-1" />);
  });
  await act(async () => {});
  return container;
}

function canvasNodeCount(container: HTMLElement) {
  return container.querySelectorAll('svg g[role="button"]').length;
}

test('graph search typing swaps only dropdown options, never canvas nodes', async () => {
  const container = await mount();
  const input = container.querySelector<HTMLInputElement>('input[role="combobox"]');
  assert.ok(input);
  assert.equal(canvasNodeCount(container), 3);
  await act(async () => input.focus());
  await act(async () => typingHelper()(input, 'man'));
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 400)); });
  const titles = [...container.querySelectorAll<HTMLButtonElement>('#wk-graph-search-results button')].map((option) => option.textContent);
  assert.deepEqual(titles, ['Manual Page'], 'remote search still fills the dropdown');
  assert.equal(canvasNodeCount(container), 3, 'typing must not filter canvas nodes (Vue WikiBrowser.vue L4680-4712 only rewrites graphSearchOptions)');
});

test('graph search debounce matches Vue: any single character fires once after 200ms', async () => {
  const { client, calls } = countingClient();
  const container = await mountWith(client);
  const input = container.querySelector<HTMLInputElement>('input[role="combobox"]');
  assert.ok(input);
  assert.equal(calls(), 0);
  await act(async () => input.focus());
  await act(async () => typingHelper()(input, 'm'));
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 120)); });
  assert.equal(calls(), 0, 'inside the 200ms debounce window no request fires');
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 250)); });
  assert.equal(calls(), 1, 'any non-empty keyword (even one char) triggers exactly one search after 200ms');
});

test('graph search empty keyword falls back to the top-500 snapshot without another request', async () => {
  const { client, calls } = countingClient();
  const container = await mountWith(client);
  const input = container.querySelector<HTMLInputElement>('input[role="combobox"]');
  assert.ok(input);
  await act(async () => input.focus());
  await act(async () => typingHelper()(input, 'ma'));
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 350)); });
  assert.equal(calls(), 1);
  let titles = [...container.querySelectorAll<HTMLButtonElement>('#wk-graph-search-results button')].map((option) => option.textContent);
  assert.deepEqual(titles, ['Manual Page']);
  await act(async () => typingHelper()(input, ''));
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 350)); });
  assert.equal(calls(), 1, 'clearing the keyword does not fire another search');
  titles = [...container.querySelectorAll<HTMLButtonElement>('#wk-graph-search-results button')].map((option) => option.textContent);
  assert.deepEqual(titles, ['Start', 'Next', 'Third'], 'empty keyword falls back to the link_count-ranked snapshot (graphSearchEffectiveOptions)');
});

test('graph legend type toggles still narrow the canvas while search stays decoupled', async () => {
  const container = await mount();
  assert.equal(canvasNodeCount(container), 3);
  const summaryToggle = [...container.querySelectorAll<HTMLButtonElement>('[data-testid="knowledge-graph-legend"] button')].find((button) => button.textContent?.trim() === '摘要');
  assert.ok(summaryToggle, 'summary legend toggle present under the default zh-CN locale');
  await act(async () => summaryToggle.click());
  await act(async () => {});
  assert.equal(canvasNodeCount(container), 2, 'disabling summary removes only the summary node — selectedTypes filtering must survive the search decoupling');
});

// kbDetail welcome tour arming: Vue KnowledgeBase.vue mounts the guide at the
// KB-page level (line 2741) with a tab-independent computed (lines 339-345),
// so a ?tab=graph deep link arms the tour too. The graph page has no document
// list request; the empty-KB signal comes from knowledge_count on the KB
// metadata this page already loads (GET /knowledge-bases/:id fills it
// server-side), so arming costs no extra request.
async function mountForGuide(client: WeKnoraClient): Promise<string[]> {
  const seen: string[] = [];
  const onEvent = (event: Event) => seen.push(String((event as CustomEvent<{ tour: string }>).detail.tour));
  window.addEventListener(OPEN_CONTEXTUAL_GUIDE_EVENT, onEvent);
  const container = document.createElement('div');
  document.body.append(container);
  const localRoot = createRoot(container);
  try {
    await act(async () => {
      localRoot.render(<KnowledgeGraphPage client={client} knowledgeBaseId="kb-1" />);
    });
    await act(async () => {});
  } finally {
    await act(async () => localRoot.unmount());
    container.remove();
    window.removeEventListener(OPEN_CONTEXTUAL_GUIDE_EVENT, onEvent);
  }
  return seen;
}

function graphClientWithKb(kbExtra: Record<string, unknown>, me: Record<string, unknown> = { user: { id: 'u-1', role: 'admin' } }): WeKnoraClient {
  const base = graphClient();
  const mutable = base as unknown as {
    knowledgeBases: { settings: { get: () => Promise<Record<string, unknown>> } };
    auth: { me: () => Promise<Record<string, unknown>> };
  };
  mutable.knowledgeBases.settings.get = async () => ({
    id: 'kb-1',
    name: 'KB',
    type: 'document',
    indexing_strategy: { wiki_enabled: true, graph_enabled: true },
    chunking_config: {},
    ...kbExtra,
  });
  mutable.auth.me = async () => me;
  return base;
}

test('kbDetail welcome tour arms on a ?tab=graph deep link for an editable empty KB', async () => {
  const seen = await mountForGuide(graphClientWithKb({ knowledge_count: 0 }));
  assert.deepEqual(seen, ['kbDetail'], 'the entry trigger fires for the empty editable KB');
  const pending = JSON.parse(window.sessionStorage.getItem(CONTEXTUAL_GUIDE_PENDING_KEY) ?? 'null') as { tour?: string } | null;
  assert.equal(pending?.tour, 'kbDetail', 'pending intent recorded for the shell guide host hand-off');
});

test('kbDetail welcome tour stays silent on the graph tab for non-empty, viewer-gated or FAQ KBs', async () => {
  for (const [label, client] of [
    ['non-empty KB', graphClientWithKb({ knowledge_count: 3 })],
    ['viewer without edit rights', graphClientWithKb({ knowledge_count: 0 }, { user: { id: 'u-2', role: 'viewer' } })],
    ['FAQ library', graphClientWithKb({ knowledge_count: 0, type: 'faq', indexing_strategy: {} })],
    // Unexpected payload without the server-filled count: stay disarmed
    // instead of guessing an empty KB.
    ['missing knowledge_count', graphClientWithKb({})],
  ] as const) {
    window.sessionStorage.clear();
    const seen = await mountForGuide(client);
    assert.deepEqual(seen, [], `no trigger for ${label}`);
    assert.equal(window.sessionStorage.getItem(CONTEXTUAL_GUIDE_PENDING_KEY), null, `no pending intent for ${label}`);
  }
});

// --- R432 W1: selection/hover edge highlight parity (Vue WikiBrowser.vue
// applyHighlight/clearHighlight L4558-4635) ---

// Fixture mirroring the wiki parity graph shape: a hub-mid reciprocal pair, a
// mid-leaf branch, and a far node whose only edge (far-hub) must stay dim
// while mid is focused.
function highlightClient(): WeKnoraClient {
  return {
    wiki: {
      graph: async () => ({
        nodes: [
          { slug: 'hub', title: 'Hub', page_type: 'summary', link_count: 5 },
          { slug: 'mid', title: 'Mid', page_type: 'entity', link_count: 2 },
          { slug: 'leaf', title: 'Leaf', page_type: 'concept', link_count: 1 },
          { slug: 'far', title: 'Far', page_type: 'synthesis', link_count: 1 },
        ],
        edges: [
          { source: 'hub', target: 'mid' },
          { source: 'mid', target: 'hub' },
          { source: 'mid', target: 'leaf' },
          { source: 'far', target: 'hub' },
        ],
        meta: { mode: 'overview', total: 4, returned: 4, truncated: false },
      }),
      list: async () => ({ pages: [], total: 0, page: 1, page_size: 20, total_pages: 0 }),
      get: async (knowledgeBaseId: string, slug: string) => ({ title: slug, summary: '', content: '# Page', version: 1, slug, knowledgeBaseId }),
    },
    knowledgeBases: {
      settings: {
        get: async () => ({
          id: 'kb-1',
          name: 'Wiki图谱fixture',
          type: 'document',
          created_at: '2026-01-02T03:04:05Z',
          indexing_strategy: { wiki_enabled: true, graph_enabled: true },
          chunking_config: {},
        }),
        parserEngines: async () => ({ data: [] }),
      },
      list: async () => [{ id: 'kb-1', name: 'Wiki图谱fixture' }],
    },
    auth: {
      me: async () => ({ user: { id: 'u-1', role: 'admin' } }),
    },
  } as unknown as WeKnoraClient;
}

function canvasSvg(container: HTMLElement) {
  return container.querySelector<SVGSVGElement>('[data-testid="knowledge-graph-surface"] > svg');
}

function nodeGroup(container: HTMLElement, title: string) {
  return container.querySelector<SVGGElement>(`svg g[role="button"][aria-label^="${title} ·"]`);
}

function mainCircle(group: SVGGElement) {
  // The main circle is the only direct circle child without aria-hidden (the
  // expansion/familiar/active rings are all aria-hidden decorations).
  return group.querySelector<SVGCircleElement>(':scope > circle:not([aria-hidden])');
}

function litLine(group: SVGGElement, ...markers: string[]) {
  return [...group.querySelectorAll<SVGLineElement>('line')].find((line) => markers.every((marker) => line.getAttribute('marker-end') === marker || line.getAttribute('marker-start') === marker));
}

test('graph defines the Vue highlight arrow markers (arrow-end-hl / arrow-start-hl, fill #0052d9)', async () => {
  const container = await mount(highlightClient());
  const markerEndHl = container.querySelector('marker#wk-graph-arrow-end-hl');
  assert.ok(markerEndHl, 'highlight end marker present');
  assert.equal(markerEndHl.getAttribute('viewBox'), '0 0 10 6');
  assert.equal(markerEndHl.getAttribute('refX'), '10');
  assert.equal(markerEndHl.getAttribute('refY'), '3');
  assert.equal(markerEndHl.getAttribute('markerWidth'), '8');
  assert.equal(markerEndHl.getAttribute('markerHeight'), '6');
  assert.equal(markerEndHl.getAttribute('orient'), 'auto');
  assert.equal(markerEndHl.querySelector('path')?.getAttribute('d'), 'M0,0 L10,3 L0,6 L2,3 Z');
  assert.equal(markerEndHl.querySelector('path')?.getAttribute('fill'), '#0052d9');
  const markerStartHl = container.querySelector('marker#wk-graph-arrow-start-hl');
  assert.ok(markerStartHl, 'highlight start marker present');
  assert.equal(markerStartHl.getAttribute('refX'), '0');
  assert.equal(markerStartHl.getAttribute('refY'), '3');
  assert.equal(markerStartHl.getAttribute('markerWidth'), '8');
  assert.equal(markerStartHl.getAttribute('markerHeight'), '6');
  assert.equal(markerStartHl.getAttribute('orient'), 'auto');
  assert.equal(markerStartHl.querySelector('path')?.getAttribute('d'), 'M10,0 L0,3 L10,6 L8,3 Z');
  assert.equal(markerStartHl.querySelector('path')?.getAttribute('fill'), '#0052d9');
  // Without hover/selection every edge keeps the plain markers (clearHighlight).
  const group = canvasSvg(container)!.querySelector('g')!;
  for (const line of group.querySelectorAll('line')) {
    assert.equal(line.getAttribute('marker-end'), 'url(#wk-graph-arrow-end)');
    assert.equal(line.getAttribute('style'), null, 'plain edges carry no inline highlight overrides');
  }
});

test('graph hover highlights incident edges and dims the rest like Vue applyHighlight', async () => {
  const container = await mount(highlightClient());
  const svg = canvasSvg(container)!;
  const group = svg.querySelector('g')!;
  const mid = nodeGroup(container, 'Mid')!;
  assert.ok(mid);
  await act(async () => mid.dispatchEvent(new dom.window.MouseEvent('mouseover', { bubbles: true })));

  // Lit edges: stroke = focus node type color (mid is entity → #2ba471),
  // opacity 0.9, width 2, highlight arrows on both ends of the hub↔mid pair.
  // (jsdom serializes the hex stroke as rgb, so accept both.)
  const hasEntityStroke = (style: string | null) => Boolean(style && (style.includes('#2ba471') || style.includes('rgb(43, 164, 113)')));
  const bidir = litLine(group, 'url(#wk-graph-arrow-end-hl)', 'url(#wk-graph-arrow-start-hl)');
  assert.ok(bidir, 'reciprocal hub-mid edge swaps both markers to the highlight arrows');
  assert.equal(hasEntityStroke(bidir.getAttribute('style')), true, 'edge stroke takes the focus node type color');
  assert.ok((bidir.getAttribute('style') ?? '').includes('stroke-opacity: 0.9'), 'lit edge opacity 0.9');
  assert.ok((bidir.getAttribute('style') ?? '').includes('stroke-width: 2'), 'lit edge width 2');
  const leafEdge = [...group.querySelectorAll('line')].find((line) => line !== bidir && (line.getAttribute('style') ?? '').includes('stroke-opacity: 0.9'));
  assert.ok(leafEdge, 'mid-leaf edge lights up too');
  assert.equal(leafEdge.getAttribute('marker-end'), 'url(#wk-graph-arrow-end-hl)');
  assert.equal(leafEdge.getAttribute('marker-start'), null, 'one-way lit edge keeps a single arrow');

  // Unrelated edge: opacity 0.08, width 1, plain markers (Vue L4612-4616).
  const dim = [...group.querySelectorAll('line')].find((line) => (line.getAttribute('style') ?? '').includes('stroke-opacity: 0.08'));
  assert.ok(dim, 'far-hub edge dims while mid is focused');
  assert.equal(dim.getAttribute('marker-end'), 'url(#wk-graph-arrow-end)');
  assert.ok((dim.getAttribute('style') ?? '').includes('stroke-width: 1'));

  // Nodes: focus grows r+3 / stroke-width 3, neighbors stay lit, the
  // unconnected node fades to opacity 0.2 (Vue L4569-4596).
  const midCircle = mainCircle(mid)!;
  assert.equal(midCircle.getAttribute('r'), String(graphNodeRadius(2) + 3), 'focus node r+3');
  assert.ok((midCircle.getAttribute('style') ?? '').includes('stroke-width: 3'), 'focus node stroke-width 3');
  const hub = nodeGroup(container, 'Hub')!;
  assert.equal(mainCircle(hub)!.getAttribute('r'), String(graphNodeRadius(5)), 'neighbor keeps its radius');
  assert.equal(hub.getAttribute('style')?.replace(' ', '').includes('opacity:1'), true, 'neighbor stays at full opacity');
  const far = nodeGroup(container, 'Far')!;
  assert.equal(far.getAttribute('style')?.replace(' ', '').includes('opacity:0.2'), true, 'unrelated node fades to opacity 0.2');

  // mouseleave debounces 60ms then falls back to plain styling (no selection).
  await act(async () => mid.dispatchEvent(new dom.window.MouseEvent('mouseout', { bubbles: true })));
  assert.equal(bidir.getAttribute('marker-end'), 'url(#wk-graph-arrow-end-hl)', 'still highlighted inside the 60ms leave debounce');
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 120)); });
  assert.equal(bidir.getAttribute('marker-end'), 'url(#wk-graph-arrow-end)', 'clearHighlight restores plain markers');
  assert.equal(bidir.getAttribute('style') ?? '', '', 'inline highlight overrides removed');
  assert.equal(far.getAttribute('style')?.replace(' ', '').includes('opacity:0.2'), false, 'dimmed node restored to full opacity');
});

test('graph tap selects (drawer + persistent highlight) and background click clears like Vue', async () => {
  const container = await mount(highlightClient());
  const svg = canvasSvg(container)!;
  const group = svg.querySelector('g')!;
  const mid = nodeGroup(container, 'Mid')!;
  // Pointer-sequence activation (pointerdown + stationary pointerup) — the
  // path real browsers take; a bare MouseEvent('click') used to mask the
  // capture bug.
  await act(async () => tapNode(mid));
  await act(async () => {});
  // Drawer opens and the selection keeps the highlight on (Vue click →
  // graphSelectedSlug + applyHighlight + openGraphDrawer).
  assert.ok(container.querySelector('aside[role="dialog"]'), 'drawer opens on tap');
  const bidir = litLine(group, 'url(#wk-graph-arrow-end-hl)', 'url(#wk-graph-arrow-start-hl)');
  assert.ok(bidir, 'selection keeps the hub-mid edge lit');
  // Vue activeRing: r+5 selection pulse ring on the selected node.
  const activeRing = mid.querySelector(':scope > circle.wk-graph-active-ring');
  assert.ok(activeRing, 'selection ring rendered');
  assert.equal(activeRing.getAttribute('r'), String(graphNodeRadius(2) + 5));

  // Closing the drawer keeps the highlight (Vue t-drawer close never clears
  // graphSelectedSlug — only a background click does).
  await act(async () => container.querySelector<HTMLElement>('aside[role="dialog"] button')!.click());
  assert.equal(container.querySelector('aside[role="dialog"]'), null, 'drawer closed');
  assert.equal(litLine(group, 'url(#wk-graph-arrow-end-hl)', 'url(#wk-graph-arrow-start-hl)'), bidir, 'highlight survives the drawer close');

  // Near-stationary background click clears selection + drawer + highlight.
  // (React delegates through bubbling, so the synthetic click must bubble;
  // the handler only acts when target === currentTarget, i.e. the svg itself.)
  await act(async () => svg.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })));
  assert.equal(bidir.getAttribute('marker-end'), 'url(#wk-graph-arrow-end)', 'background click restores plain markers');
  assert.equal(mid.querySelector(':scope > circle.wk-graph-active-ring'), null, 'selection ring cleared');
  assert.equal(container.querySelector('aside[role="dialog"]'), null);
});

test('graph node drag never activates the node — tap is the only pointer activation path', async () => {
  const container = await mount(highlightClient());
  const svg = canvasSvg(container)!;
  const mid = nodeGroup(container, 'Mid')!;
  // jsdom reports an empty rect; pin it so pointer coordinates map to svg
  // space and the >3px drag threshold actually trips.
  Object.defineProperty(svg, 'getBoundingClientRect', {
    configurable: true,
    value: () => ({ x: 0, y: 0, left: 0, top: 0, right: 760, bottom: 420, width: 760, height: 420, toJSON: () => ({}) }),
  });
  await act(async () => {
    mid.dispatchEvent(new dom.window.PointerEvent('pointerdown', { bubbles: true, pointerId: 7, button: 0, clientX: 50, clientY: 50 }));
    svg.dispatchEvent(new dom.window.PointerEvent('pointermove', { bubbles: true, pointerId: 7, button: 0, clientX: 120, clientY: 90 }));
    svg.dispatchEvent(new dom.window.PointerEvent('pointerup', { bubbles: true, pointerId: 7, button: 0, clientX: 120, clientY: 90 }));
  });
  await act(async () => {});
  assert.equal(container.querySelector('aside[role="dialog"]'), null, 'a real drag never opens the drawer');
  assert.equal(mid.querySelector(':scope > circle.wk-graph-active-ring'), null, 'a real drag never selects the node');
  const dimmedLines = [...svg.querySelectorAll('line')].filter((line) => (line.getAttribute('style') ?? '').includes('stroke-opacity'));
  assert.equal(dimmedLines.length, 0, 'a real drag leaves the highlight state untouched');
});

test('graph double-tap on the same node fetches the drawer page once (Vue pendingSingleClick)', async () => {
  const client = highlightClient();
  let pageFetches = 0;
  const innerGet = client.wiki.get.bind(client.wiki);
  client.wiki.get = (async (...args: Parameters<typeof innerGet>) => {
    pageFetches += 1;
    return innerGet(...args);
  }) as typeof client.wiki.get;
  const container = await mountWith(client);
  const mid = nodeGroup(container, 'Mid')!;
  await act(async () => tapNode(mid, 2));
  await act(async () => tapNode(mid, 3));
  await act(async () => {});
  assert.equal(pageFetches, 1, 'the second tap inside the 300ms window belongs to the dblclick ego pivot, not another fetch');
  assert.ok(container.querySelector('aside[role="dialog"]'), 'the first tap still opened the drawer');
});

test('graph keyboard selection produces the same highlight as Vue applyHighlight', async () => {
  const container = await mount(highlightClient());
  const group = canvasSvg(container)!.querySelector('g')!;
  const mid = nodeGroup(container, 'Mid')!;
  await act(async () => mid.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })));
  await act(async () => {});
  const bidir = litLine(group, 'url(#wk-graph-arrow-end-hl)', 'url(#wk-graph-arrow-start-hl)');
  assert.ok(bidir, 'keyboard selection lights the incident edges');
  assert.ok(mid.querySelector(':scope > circle.wk-graph-active-ring'), 'keyboard selection shows the selection ring');
  assert.ok(container.querySelector('aside[role="dialog"]'), 'keyboard selection opens the drawer like Enter on the Vue canvas');
});
