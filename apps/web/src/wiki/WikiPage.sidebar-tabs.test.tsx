// Wiki sidebar tab count regression tests (Vue parity): Vue WikiBrowser's
// groupedPages total (WikiBrowser.vue total = bucket.total || statTotal(tab))
// always reflects the whole-library /wiki/stats pages_by_type, independent of
// the sidebar view mode. The React port used to count only root-level pages of
// the first loaded batch in tree view, so the 知识 tab vanished whenever that
// batch held no root-level entity/concept pages (only 摘要 survived).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
hooks.registerHooks?.({ resolve: (specifier, context, nextResolve) => (specifier.endsWith('.css') || specifier.endsWith('.svg')) ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

import * as React from 'react';

import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/knowledgeBase/kb-1/wiki' });
// TenantMembersPanel.test.tsx 判例：tdesign Popup/Tooltip 等按自由标识符读
// Element/HTMLElement 等全局，挂上 jsdom 的全量 DOM 类后才能 createRoot 挂载。
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  FocusEvent: dom.window.FocusEvent,
  NodeFilter: dom.window.NodeFilter,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  KeyboardEvent: dom.window.KeyboardEvent,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((callback: FrameRequestCallback) => setTimeout(callback, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');

const { WikiPage } = await import('./WikiPage.tsx');

// First batch: a root-level summary page plus an entity page nested under a
// folder — no ROOT-level knowledge pages, the exact shape that used to make
// the 知识 tab disappear in tree view. Stats still report the whole library.
function makeClient(pagesByType: Record<string, number>) {
  return {
    wiki: {
      list: async () => ({
        pages: [
          { slug: 'summary/root-1', title: '总结', page_type: 'summary', category_path: [] },
          { slug: 'entity/nested-1', title: '嵌套实体', page_type: 'entity', category_path: ['领域'] },
        ],
        total: 2,
      }),
      stats: async () => ({ pages_by_type: pagesByType }),
      index: async () => ({ intro: '', version: 0, groups: [] }),
      folders: async () => ({ folders: [] }),
    },
    knowledgeBases: {
      list: async () => [],
      settings: {
        get: async () => ({ id: 'kb-1', name: '知识库', indexing_strategy: { wiki_enabled: true } }),
        parserEngines: async () => ({ data: [] }),
      },
    },
    auth: { me: async () => null },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => null } } },
  };
}

function tabCount(container: HTMLElement, label: string): string {
  const tab = [...container.querySelectorAll<HTMLElement>('[role="tab"]')].find((element) => (element.textContent ?? '').includes(label));
  assert.ok(tab, `expected the ${label} tab, got ${[...container.querySelectorAll('[role="tab"]')].map((element) => element.textContent).join(',')}`);
  const count = tab.querySelector<HTMLElement>('.wiki-tab-count');
  assert.ok(count, `the ${label} tab must render its count`);
  return count.textContent ?? '';
}

test('tree-view sidebar tabs carry the whole-library stats counts (Vue groupedPages parity)', async () => {
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  // entity 3 + concept 2 = 知识 5; summary 7 — none of the root-level loaded
  // pages contribute to the knowledge bucket.
  await React.act(async () => {
    root.render(React.createElement(WikiPage, { client: makeClient({ entity: 3, concept: 2, synthesis: 0, comparison: 0, summary: 7 }) as never, knowledgeBaseId: 'kb-1', canContribute: false }));
    await new Promise((resolve) => setTimeout(resolve, 20));
  });
  // Default view is tree: assert the fix applies in the mode that regressed.
  assert.equal(container.querySelector<HTMLElement>('button[aria-label="树形视图"]')?.getAttribute('aria-pressed'), 'true', 'tree view is the default sidebar mode');
  assert.equal(tabCount(container, '知识'), '5', '知识 counts the whole library from /wiki/stats, not first-batch root pages');
  assert.equal(tabCount(container, '摘要'), '7');
  // Same counts after switching to list view (Vue keeps totals view-agnostic).
  const listToggle = container.querySelector<HTMLElement>('button[aria-label="列表视图"]');
  assert.ok(listToggle, 'the list view toggle renders');
  await React.act(async () => {
    listToggle.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true }));
    await new Promise((resolve) => setTimeout(resolve, 20));
  });
  assert.equal(tabCount(container, '知识'), '5', 'both views show the same whole-library count');
  assert.equal(tabCount(container, '摘要'), '7');
  await React.act(async () => { root.unmount(); });
  container.remove();
});

test('tabs with a zero whole-library count stay hidden in both views', async () => {
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  await React.act(async () => {
    root.render(React.createElement(WikiPage, { client: makeClient({ entity: 0, concept: 0, synthesis: 0, comparison: 0, summary: 7 }) as never, knowledgeBaseId: 'kb-1', canContribute: false }));
    await new Promise((resolve) => setTimeout(resolve, 20));
  });
  const labels = [...container.querySelectorAll<HTMLElement>('[role="tab"]')].map((element) => element.textContent);
  assert.ok(!labels.some((label) => (label ?? '').includes('知识')), `a zero-count knowledge tab must stay hidden, got ${labels.join(',')}`);
  assert.equal(tabCount(container, '摘要'), '7');
  await React.act(async () => { root.unmount(); });
  container.remove();
});
