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

const { WikiPage, wikiFormatDate } = await import('./WikiPage.tsx');

// First batch: a root-level summary page plus an entity page nested under a
// folder — no ROOT-level knowledge pages, the exact shape that used to make
// the 知识 tab disappear in tree view. Stats still report the whole library.
function makeClient(pagesByType: Record<string, number>) {
  return {
    // KBW-4：stats 由轮询 hook 走 client.request 原始通道拉取（保留
    // pending_tasks/is_active），mock 按路径应答；空闲态不触发后续轮询。
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/knowledgebase/kb-1/wiki/stats') {
        return { total_pages: 0, pages_by_type: pagesByType, pending_tasks: 0, is_active: false, pending_issues: 0 };
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
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

// KBW-8：Vue 列表视图行三段式（WikiBrowser.vue L358-366）——标题 + 摘要 +
// formatDate(updated_at)。React 此前只有图标+标题；补齐摘要与更新日期，
// 日期口径复用 wikiFormatDate（YYYY/MM/DD HH:mm:ss，与 Vue formatDate 一致）。
test('list-view rows render title, summary and updated date (Vue three-segment parity)', async () => {
  const updated_at = '2026-09-28T10:20:30Z';
  const client = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/knowledgebase/kb-1/wiki/stats') {
        return { total_pages: 0, pages_by_type: { entity: 1, concept: 0, synthesis: 0, comparison: 0, summary: 1 }, pending_tasks: 0, is_active: false, pending_issues: 0 };
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
    wiki: {
      list: async () => ({
        pages: [
          { slug: 'entity/acme', title: 'ACME 条目', page_type: 'entity', summary: '关于 ACME 的实体摘要。', updated_at, category_path: [] },
          { slug: 'summary/root-1', title: '总结', page_type: 'summary', summary: '全库总结摘要。', updated_at, category_path: [] },
        ],
        total: 2,
      }),
      stats: async () => ({ pages_by_type: { entity: 1, concept: 0, synthesis: 0, comparison: 0, summary: 1 } }),
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
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  await React.act(async () => {
    root.render(React.createElement(WikiPage, { client: client as never, knowledgeBaseId: 'kb-1', canContribute: false }));
    await new Promise((resolve) => setTimeout(resolve, 20));
  });
  const listToggle = container.querySelector<HTMLElement>('button[aria-label="列表视图"]');
  assert.ok(listToggle, 'the list view toggle renders');
  await React.act(async () => {
    listToggle.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true }));
    await new Promise((resolve) => setTimeout(resolve, 20));
  });
  const rows = [...container.querySelectorAll<HTMLElement>('.wk-wiki-page-item.wiki-page-item--list')];
  assert.ok(rows.length > 0, `list view renders three-segment rows, got ${container.innerHTML.slice(0, 200)}`);
  const expectedDate = wikiFormatDate(updated_at);
  assert.ok(expectedDate.length > 0, 'the fixture timestamp must format');
  for (const row of rows) {
    assert.ok(row.querySelector('.wiki-page-item-summary'), 'each row carries the summary segment');
    assert.ok(row.querySelector('.wiki-page-item-meta'), 'each row carries the date segment');
    assert.ok((row.textContent ?? '').includes(expectedDate), `row shows formatDate(updated_at)=${expectedDate}`);
  }
  const acme = rows.find((row) => (row.textContent ?? '').includes('ACME 条目'));
  assert.ok(acme, 'the entity row renders in list view');
  assert.ok((acme.textContent ?? '').includes('关于 ACME 的实体摘要。'), 'the row shows the page summary');
  await React.act(async () => { root.unmount(); });
  container.remove();
});

// KBW-4：索引中（is_active || pending_tasks>0）时 Wiki/图谱面包屑 tab 点亮
// indexing 态——tab 加 indexing class，label 后渲染 t-loading 小指示器
// （Vue KnowledgeBase.vue L2426-2440 wikiIsIndexing 分支）。
test('indexing stats light up the wiki/graph breadcrumb tabs', async () => {
  const client = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/knowledgebase/kb-1/wiki/stats') {
        return { total_pages: 0, pages_by_type: { entity: 1, concept: 0, synthesis: 0, comparison: 0, summary: 0 }, pending_tasks: 3, is_active: true, pending_issues: 0 };
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
    wiki: {
      list: async () => ({ pages: [], total: 0 }),
      stats: async () => ({ pages_by_type: { entity: 1, concept: 0, synthesis: 0, comparison: 0, summary: 0 } }),
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
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  await React.act(async () => {
    root.render(React.createElement(WikiPage, { client: client as never, knowledgeBaseId: 'kb-1', canContribute: false }));
    await new Promise((resolve) => setTimeout(resolve, 20));
  });
  const tabs = [...container.querySelectorAll<HTMLElement>('.breadcrumb-tab')];
  const wikiTab = tabs.find((tab) => tab.textContent?.startsWith('Wiki'));
  assert.ok(wikiTab, 'the wiki breadcrumb tab renders');
  assert.match(wikiTab.className, /(^| )indexing( |$)/, 'indexing stats add the indexing class to the wiki tab');
  assert.ok(wikiTab.querySelector('.breadcrumb-tab-indicator'), 'the indexing tab carries the loading indicator');
  assert.ok(wikiTab.querySelector('.t-loading'), 'the indicator is a tdesign t-loading');
  const docsTab = tabs.find((tab) => tab.textContent && !tab.textContent.startsWith('Wiki') && !tab.textContent.includes('图'));
  assert.ok(docsTab, 'the documents breadcrumb tab renders');
  assert.doesNotMatch(docsTab.className, /indexing/, 'the documents tab never shows the indexing state');
  await React.act(async () => { root.unmount(); });
  container.remove();
});
