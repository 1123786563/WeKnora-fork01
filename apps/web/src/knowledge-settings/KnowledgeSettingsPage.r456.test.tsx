// R456 A1 round: the parser/vectorStore/storage summary tiles still rendered
// hardcoded English on non-en locales ("Default parser", "No file-type
// overrides", "System default", "No explicit binding", "No explicit
// instance", …). This round adds fresh kbSettings.summary.* keys for the
// non-data branches (data-driven literals stay the en-US fallbacks) and
// wires them through the same labelKey/detailKey mechanism R455 introduced.
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
  id: 'kb-456',
  name: 'Product docs',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
};

// ---- unit: the data-driven branches keep undefined keys, fallbacks gain keys --

test('summarizeKnowledgeSettings keeps data-driven literals key-free and keys the fallback branches', () => {
  const configured = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    chunking_config: { parser_engine_rules: [{ file_types: ['pdf'], engine: 'mineru' }] },
    vector_store_name: 'weaviate-prod',
    vector_store_engine_type: 'weaviate',
    vector_store_source: 'global',
    storage_backend_id: 'store-1',
    storage_provider_config: { provider: 'local' },
  });
  assert.equal(configured.parser.labelKey, undefined);
  assert.equal(configured.parser.detailKey, undefined);
  assert.equal(configured.vectorStore.labelKey, undefined);
  assert.equal(configured.vectorStore.detailKey, undefined);
  assert.equal(configured.storage.labelKey, undefined);
  assert.equal(configured.storage.detailKey, undefined);

  const rulesWithoutEngine = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    chunking_config: { parser_engine_rules: [{ file_types: ['pdf'] }] },
  });
  assert.equal(rulesWithoutEngine.parser.labelKey, 'kbSettings.summary.parser.customRulesLabel');
  assert.equal(rulesWithoutEngine.parser.detailKey, undefined);

  const rulesWithoutFileTypes = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    chunking_config: { parser_engine_rules: [{ engine: 'builtin' }] },
  });
  assert.equal(rulesWithoutFileTypes.parser.labelKey, undefined);
  assert.equal(rulesWithoutFileTypes.parser.detailKey, 'kbSettings.summary.parser.overridesDetail');

  const unavailableVector = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    vector_store_status: 'unavailable',
    vector_store_name: 'weaviate-prod',
  });
  assert.equal(unavailableVector.vectorStore.labelKey, undefined);
  assert.equal(unavailableVector.vectorStore.detailKey, 'kbSettings.summary.vectorStore.unavailableDetail');

  const unnamedVector = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    vector_store_status: 'unavailable',
  });
  assert.equal(unnamedVector.vectorStore.labelKey, 'kbSettings.summary.vectorStore.unavailableLabel');

  const boundWithoutName = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    vector_store_id: 'vec-1',
  });
  assert.equal(boundWithoutName.vectorStore.labelKey, 'kbSettings.summary.vectorStore.boundLabel');
  assert.equal(boundWithoutName.vectorStore.detailKey, 'kbSettings.summary.vectorStore.explicitDetail');

  const boundWithEngine = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    vector_store_id: 'vec-1',
    vector_store_engine_type: 'weaviate',
  });
  assert.equal(boundWithEngine.vectorStore.detailKey, undefined);

  const providerWithoutId = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    storage_provider_config: { provider: 'local' },
  });
  assert.equal(providerWithoutId.storage.labelKey, undefined);
  assert.equal(providerWithoutId.storage.detailKey, 'kbSettings.summary.storage.providerDetail');

  const idWithoutProvider = summarizeKnowledgeSettings({
    ...baseKnowledgeBase,
    storage_backend_id: 'store-1',
  });
  assert.equal(idWithoutProvider.storage.labelKey, 'kbSettings.summary.storage.instanceLabel');
  assert.equal(idWithoutProvider.storage.detailKey, undefined);
});

// ---- render (R484): the overview tiles are gone from the drawer; the
// summary only feeds the vectorStore/storage control labels ---------------

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

async function renderSection(section: string, kb: KnowledgeSettingsInput = baseKnowledgeBase, locale = 'zh-CN'): Promise<string> {
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
  return document.body.textContent ?? '';
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

test('zh-CN parser tab renders the Vue per-file-type list instead of the summary tile', async () => {
  const bodyText = await renderSection('parser');
  assert.ok(!bodyText.includes('默认解析引擎'), `the parser tile label must be gone, got: ${JSON.stringify(bodyText.slice(0, 400))}`);
  assert.ok(!bodyText.includes('未按文件类型覆盖'), 'the parser tile detail must be gone');
  assert.ok(!bodyText.includes('Default parser'), 'no hardcoded English either');
  assert.ok(!bodyText.includes('No file-type overrides'), 'no hardcoded English detail either');
});

test('zh-CN parser tab renders no custom-rules tile fallback', async () => {
  const customRules: KnowledgeSettingsInput = {
    ...baseKnowledgeBase,
    // A rule without engine or file types resolves the old fallback branches.
    chunking_config: { parser_engine_rules: [{}] },
  };
  const bodyText = await renderSection('parser', customRules);
  assert.ok(!bodyText.includes('自定义规则'), 'the custom-rules tile label must be gone');
  assert.ok(!bodyText.includes('按文件类型覆盖'), 'the file-type overrides tile detail must be gone');
  assert.ok(!bodyText.includes('Custom rules'), 'no hardcoded English either');
});

test('zh-CN vectorStore select keeps the localized label, the tile detail is gone', async () => {
  const bodyText = await renderSection('vectorStore');
  assert.ok(bodyText.includes('系统默认'), 'the disabled select carries the localized default label');
  assert.ok(!bodyText.includes('未显式绑定'), 'the vector no-binding tile detail must be gone');
  assert.ok(!bodyText.includes('No explicit binding'), 'no hardcoded English detail either');

  const unavailable: KnowledgeSettingsInput = { ...baseKnowledgeBase, vector_store_status: 'unavailable' };
  const unavailableText = await renderSection('vectorStore', unavailable);
  assert.ok(!unavailableText.includes('请检查全局向量存储设置'), 'the vector unavailable tile detail must be gone');

  const bound: KnowledgeSettingsInput = { ...baseKnowledgeBase, vector_store_id: 'vec-1' };
  const boundText = await renderSection('vectorStore', bound);
  assert.ok(boundText.includes('已绑定的向量存储'), 'the bound label feeds the disabled select');
  assert.ok(!boundText.includes('显式绑定'), 'the explicit-binding tile detail must be gone');
});

test('zh-CN storage select keeps its labels, the tile detail is gone', async () => {
  const bodyText = await renderSection('storage');
  assert.ok(!bodyText.includes('未绑定实例'), 'the storage no-instance tile detail must be gone');
  assert.ok(!bodyText.includes('No explicit instance'), 'no hardcoded English detail either');

  const providerOnly: KnowledgeSettingsInput = {
    ...baseKnowledgeBase,
    storage_provider_config: { provider: 'local' },
  };
  const providerText = await renderSection('storage', providerOnly);
  assert.ok(!providerText.includes('已配置存储提供方'), 'the provider-configured tile detail must be gone');
});

test('en-US renders no parser/vectorStore/storage tile copy', async () => {
  const bodyText = await renderSection('parser', baseKnowledgeBase, 'en-US');
  assert.ok(!bodyText.includes('Default parser') && !bodyText.includes('No file-type overrides'), 'the parser tile copy is gone');

  const vectorText = await renderSection('vectorStore', baseKnowledgeBase, 'en-US');
  assert.ok(vectorText.includes('System default'), 'the disabled select keeps the localized default label');
  assert.ok(!vectorText.includes('No explicit binding'), 'the vector tile detail is gone');

  const storageText = await renderSection('storage', baseKnowledgeBase, 'en-US');
  assert.ok(!storageText.includes('No explicit instance'), 'the storage tile detail is gone');

  const customRules: KnowledgeSettingsInput = {
    ...baseKnowledgeBase,
    chunking_config: { parser_engine_rules: [{}] },
  };
  const customText = await renderSection('parser', customRules, 'en-US');
  assert.ok(!customText.includes('Custom rules') && !customText.includes('File-type overrides'), 'the parser configured tile copy is gone');
});
