// R455 A1 round added the knowledge-settings summary tiles (the activity /
// datasource / share / graph overview cards) with kbSettings.summary.* keys.
// R484 (R482 B1 差异4) removed the tiles from the drawer — Vue's settings
// drawer carries no overview cards, and their copy surfaced as parity diffs
// (「暂无数据源/添加外部数据源/未共享/知识图谱未启用」). The pure
// summarizeKnowledgeSettings keeps its structured keys (still consumed by
// the vectorStore/storage controls) and the section empty sentences stay;
// this file now pins both the pure contract and the tile-free drawer.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
dom.window.localStorage.setItem('locale', 'zh-CN');
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLElement,
  HTMLButtonElement: dom.window.HTMLButtonElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  PointerEvent: dom.window.PointerEvent,
  NodeFilter: dom.window.NodeFilter,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { KnowledgeSettingsPage, summarizeKnowledgeSettings } = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

const baseKnowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-455',
  name: 'Product docs',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
};

// ---- unit: the pure summary exposes structured i18n message keys -------------

test('summarizeKnowledgeSettings exposes summary message keys with {count} params for the four overview tiles', () => {
  const empty = summarizeKnowledgeSettings(baseKnowledgeBase);
  assert.equal(empty.activity.labelKey, 'kbSettings.summary.activity.emptyLabel');
  assert.equal(empty.activity.detailKey, 'kbSettings.summary.activity.emptyDetail');
  assert.equal(empty.datasource.labelKey, 'kbSettings.summary.datasource.emptyLabel');
  assert.equal(empty.datasource.detailKey, 'kbSettings.summary.datasource.emptyDetail');
  assert.equal(empty.share.labelKey, 'kbSettings.summary.share.emptyLabel');
  assert.equal(empty.share.detailKey, 'kbSettings.summary.share.emptyDetail');
  assert.equal(empty.graph.labelKey, 'kbSettings.summary.graph.disabledLabel');
  assert.equal(empty.graph.detailKey, 'kbSettings.summary.graph.disabledDetail');
  // R456: the parser/vectorStore/storage tiles gained summary keys for the
  // non-data branches (the data-driven literals stay the en-US fallbacks).
  // The empty KB below resolves every tile to its default/empty branch.
  assert.equal(empty.parser.labelKey, 'kbSettings.summary.parser.defaultLabel');
  assert.equal(empty.parser.detailKey, 'kbSettings.summary.parser.noOverridesDetail');
  assert.equal(empty.vectorStore.labelKey, 'kbSettings.summary.vectorStore.defaultLabel');
  assert.equal(empty.vectorStore.detailKey, 'kbSettings.summary.vectorStore.noBindingDetail');
  assert.equal(empty.storage.labelKey, 'kbSettings.summary.storage.defaultLabel');
  assert.equal(empty.storage.detailKey, 'kbSettings.summary.storage.noInstanceDetail');

  const populated = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    activity_count: 1,
    data_source_count: 2,
    share_count: 3,
    extract_config: { enabled: true },
  });
  assert.equal(populated.activity.labelKey, 'kbSettings.summary.activity.countOne');
  assert.deepEqual(populated.activity.labelParams, { count: 1 });
  assert.equal(populated.datasource.labelKey, 'kbSettings.summary.datasource.countOther');
  assert.deepEqual(populated.datasource.labelParams, { count: 2 });
  assert.equal(populated.datasource.detailKey, 'kbSettings.summary.datasource.inspect');
  assert.equal(populated.share.labelKey, 'kbSettings.summary.share.countOther');
  assert.deepEqual(populated.share.labelParams, { count: 3 });
  assert.equal(populated.share.detailKey, 'kbSettings.summary.share.managed');
  assert.equal(populated.graph.labelKey, 'kbSettings.summary.graph.enabledLabel');
  assert.equal(populated.graph.detailKey, 'kbSettings.summary.graph.enabledDetail');

  const pluralSingular = summarizeKnowledgeSettings({ ...baseKnowledgeBase, data_source_count: 1 });
  assert.equal(pluralSingular.datasource.labelKey, 'kbSettings.summary.datasource.countOne');
  const activityFallback = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    activity: [],
    activity_count: 5,
  });
  assert.equal(activityFallback.activity.labelKey, 'kbSettings.summary.activity.countOther');
  assert.equal(activityFallback.activity.detailKey, 'kbSettings.summary.activity.inspect');
});

// ---- render: the drawer renders no overview tiles (R484 Vue parity) ----

test('zh-CN drawer renders no datasource/share/graph overview tiles', async () => {
  await renderSection('datasource');
  let bodyText = document.body.textContent ?? '';
  assert.ok(!bodyText.includes('暂无数据源'), `the datasource tile empty label must be gone, got: ${JSON.stringify(bodyText.slice(0, 400))}`);
  assert.ok(!bodyText.includes('添加外部数据源'), 'the datasource tile empty detail must be gone');
  assert.ok(!bodyText.includes('No data sources'), 'no hardcoded English either');

  await renderSection('share');
  bodyText = document.body.textContent ?? '';
  assert.ok(!bodyText.includes('尚无空间获得访问权限'), 'the share tile empty detail must be gone');
  assert.ok(!bodyText.includes('No spaces have access'), 'no hardcoded English tile detail either');

  await renderSection('graph');
  bodyText = document.body.textContent ?? '';
  assert.ok(!bodyText.includes('知识图谱未启用'), 'the graph tile disabled label must be gone');
  assert.ok(!bodyText.includes('Knowledge graph disabled'), 'no hardcoded English either');
});

test('populated sections render no {count} overview tiles either', async () => {
  const kb: KnowledgeSettingsInput = {
    ...baseKnowledgeBase,
    data_source_count: 2,
    share_count: 3,
    extract_config: { enabled: true },
  };
  await renderSection('datasource', kb);
  let bodyText = document.body.textContent ?? '';
  assert.ok(!bodyText.includes('2 个数据源'), 'the datasource count tile must be gone');
  assert.ok(!bodyText.includes('查看同步状态'), 'the datasource inspect tile detail must be gone');

  await renderSection('share', kb);
  bodyText = document.body.textContent ?? '';
  assert.ok(!bodyText.includes('3 个共享空间'), 'the share count tile must be gone');
  assert.ok(!bodyText.includes('访问权限按共享空间单独管理'), 'the share managed tile detail must be gone');

  await renderSection('graph', kb);
  bodyText = document.body.textContent ?? '';
  assert.ok(!bodyText.includes('知识图谱已启用'), 'the graph enabled tile must be gone');
});

test('en-US drawer renders no overview tile copy', async () => {
  const kb: KnowledgeSettingsInput = {
    ...baseKnowledgeBase,
    data_source_count: 2,
    share_count: 3,
    extract_config: { enabled: true },
  };
  await renderSection('datasource', kb, 'en-US');
  let bodyText = document.body.textContent ?? '';
  assert.ok(!bodyText.includes('2 data sources'), 'the en datasource count tile must be gone');
  assert.ok(!bodyText.includes('Open to inspect sync status'), 'the en datasource inspect tile detail must be gone');
  assert.ok(!bodyText.includes('Add an external data source'), 'the en datasource empty tile detail must be gone');

  await renderSection('graph', kb, 'en-US');
  bodyText = document.body.textContent ?? '';
  assert.ok(!bodyText.includes('Knowledge graph enabled'), 'the en graph enabled tile must be gone');
  assert.ok(!bodyText.includes('Entity and relationship extraction'), 'the en graph tile detail must be gone');
});

function clientFor(): WeKnoraClient {
  const request = async () => ({ success: true });
  return {
    request,
    configuration: { models: { list: async () => [] } },
    dataSources: {
      list: async () => [],
      types: async () => [],
    },
    knowledgeBases: {
      documents: { list: async () => ({ data: [], total: 0 }) },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
        activity: async () => ({ success: true, data: [] }),
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderSection(section: string, kb: KnowledgeSettingsInput = baseKnowledgeBase, locale = 'zh-CN'): Promise<void> {
  dom.window.localStorage.setItem('locale', locale);
  if (mountedRoot) {
    const previous = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { previous.unmount(); });
    document.body.innerHTML = '';
  }
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client: clientFor(), knowledgeBase: kb, role: 'admin', initialSection: section as never }));
  });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

async function renderFallbackSections(section: string, locale = 'zh-CN'): Promise<void> {
  dom.window.localStorage.setItem('locale', locale);
  if (mountedRoot) {
    const previous = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { previous.unmount(); });
    document.body.innerHTML = '';
  }
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  // No client: the activity/datasource/share sections fall back to the
  // hardcoded empty sentences under test.
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { knowledgeBase: baseKnowledgeBase, role: 'admin', initialSection: section as never }));
  });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
  dom.window.localStorage.setItem('locale', 'zh-CN');
});

test('zh-CN translates the activity/datasource/share section empty sentences', async () => {
  await renderFallbackSections('activity');
  let bodyText = document.body.textContent ?? '';
  assert.ok(bodyText.includes('该知识库暂无变更记录。'), 'expected the zh activity empty sentence');
  assert.ok(!bodyText.includes('No recorded changes for this knowledge base.'), 'the activity fallback must not render hardcoded English');

  await renderFallbackSections('datasource');
  bodyText = document.body.textContent ?? '';
  assert.ok(bodyText.includes('尚未配置数据源。'), 'expected the zh datasource empty sentence');
  assert.ok(!bodyText.includes('No data sources configured.'), 'the datasource fallback must not render hardcoded English');

  await renderFallbackSections('share');
  bodyText = document.body.textContent ?? '';
  assert.ok(bodyText.includes('该知识库尚未共享。'), 'expected the zh share empty sentence');
  assert.ok(!bodyText.includes('This knowledge base is not shared.'), 'the share fallback must not render hardcoded English');
});
