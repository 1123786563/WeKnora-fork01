// R484 G2 — graph empty-state gating, parser warning banner, and the hosted
// settings Dialog seeding, split out of KnowledgeGraphPage.test.tsx: the
// 13-test single file hangs node:test inside the empty-state mount (runner
// interaction, not a product defect — every test passes in isolation and in
// partial-order runs; only the full 13-test sequence stalls). Same harness,
// same fixtures, coverage unchanged.
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
  Event: dom.window.Event,
  PointerEvent: dom.window.PointerEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { KnowledgeGraphPage } = await import('./KnowledgeGraphPage.tsx');

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
    },
    knowledgeBases: {
      settings: {
        get: async () => ({
          id: 'kb-1',
          name: 'Wiki图谱fixture',
          type: 'document',
          indexing_strategy: { wiki_enabled: true, graph_enabled: true },
          chunking_config: {},
        }),
        parserEngines: async () => ({ data: [{ Name: 'builtin', FileTypes: ['pdf', 'md'], Available: true }] }),
      },
      list: async () => [{ id: 'kb-1', name: 'Wiki图谱fixture' }],
    },
    auth: { me: async () => ({ user: { id: 'u-1', role: 'admin' } }) },
  } as unknown as WeKnoraClient;
}

function emptyGraphClient(): WeKnoraClient {
  const client = graphClient();
  const mutable = client as unknown as { wiki: { graph: () => Promise<unknown> } };
  mutable.wiki.graph = async () => ({
    nodes: [],
    edges: [],
    meta: { mode: 'overview', total: 0, returned: 0, truncated: false },
  });
  return client;
}

// Vue KnowledgeBase.vue L2458-2465: when tenant engines leave some advertised
// file types without any available engine, every KB detail tab (graph
// included) renders the .parser-hint warning line with a 前往配置 → link.
function partialEngineClient(): WeKnoraClient {
  const client = graphClient();
  const mutable = client as unknown as {
    knowledgeBases: { settings: { parserEngines: () => Promise<{ data: unknown }> } };
  };
  mutable.knowledgeBases.settings.parserEngines = async () => ({
    data: [
      { Name: 'builtin', FileTypes: ['pdf', 'md'], Available: true },
      { Name: 'office', FileTypes: ['docm', 'odp'], Available: false },
    ],
  });
  return client;
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

test('graph empty state hides the whole toolbar until nodes render (Vue graphReady)', async () => {
  const container = await mountWith(emptyGraphClient());
  assert.match(container.querySelector('.wiki-graph-empty')?.textContent ?? '', /暂无图谱数据/);
  assert.equal(container.querySelector('input[role="combobox"]'), null, 'search control hidden on the empty graph');
  assert.equal(container.querySelector('details'), null, '? help trigger hidden on the empty graph');
  assert.equal(container.querySelector('[data-testid="knowledge-graph-legend"]'), null, 'legend hidden on the empty graph');
  assert.doesNotMatch(container.textContent ?? '', /个节点/, 'node count status card hidden');
  assert.doesNotMatch(container.textContent ?? '', /已展示知识库全部节点/, 'overview hint hidden');
  assert.doesNotMatch(container.textContent ?? '', /适应屏幕|隐藏箭头/, 'legend actions hidden');
  assert.doesNotMatch(container.textContent ?? '', /摘要|概念/, 'type filter chips hidden');
});

test('graph toolbar reappears once nodes exist (graphReady flips with data)', async () => {
  const container = await mountWith(graphClient());
  assert.ok(container.querySelector('input[role="combobox"]'), 'search control present with nodes');
  assert.ok(container.querySelector('[data-testid="knowledge-graph-legend"]'), 'legend present with nodes');
});

test('graph header renders the parser-engine warning banner with 前往配置', async () => {
  const container = await mountWith(partialEngineClient());
  const hint = container.querySelector('.parser-hint');
  assert.ok(hint, 'parser-hint warning line renders under the subtitle');
  assert.match(hint?.textContent ?? '', /部分文档类型（\.docm、\.odp）暂无可用解析引擎，上传后将无法解析/);
  assert.match(hint?.textContent ?? '', /前往配置/);
  assert.ok(container.querySelector('.document-subtitle'), 'document subtitle still present');
});

test('graph parser banner 前往配置 opens KB settings on the parser section (Vue openKBSettings parser)', async () => {
  await mountWith(partialEngineClient());
  const source = KnowledgeGraphPage.toString();
  assert.match(source, /settingsSection/, 'the graph page tracks which settings section to open');
  assert.match(source, /initialSection[=:]\s*\{?settingsSection/, 'the hosted settings Dialog is seeded with that section');
  assert.match(source, /setSettingsSection\("parser"\)|setSettingsSection\('parser'\)/, 'the parser banner routes to the parser section');
  assert.match(source, /onConfigure[=:]\s*\{?openParserSettings/, 'the parser-hint banner click routes through openParserSettings');
});

test('graph parser banner stays hidden when every advertised type has an engine', async () => {
  const container = await mountWith(graphClient());
  assert.equal(container.querySelector('.parser-hint'), null, 'no unresolved types — no banner');
});
