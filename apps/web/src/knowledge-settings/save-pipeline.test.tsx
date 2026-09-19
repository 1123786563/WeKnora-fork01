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
dom.window.localStorage.setItem('locale', 'en-US');
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
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
const {
  KnowledgeSettingsPage,
  buildKnowledgeSettingsConfigPayload,
  getKnowledgeBaseConfigPath,
  saveKnowledgeSettings,
} = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

// Mirrors a knowledge-base row as returned by GET /api/v1/knowledge-bases/:id
// (open-ended contract passthrough, same fields the Vue editor reads).
const knowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
  chunking_config: {
    chunk_size: 700,
    chunk_overlap: 90,
    separators: ['\n\n', '\n'],
    enable_parent_child: true,
    parent_chunk_size: 4096,
    child_chunk_size: 384,
    strategy: 'auto',
    token_limit: 0,
    languages: ['zh'],
    table_metadata_instructions: 'keep headers',
    parser_engine_rules: [{ file_types: ['pdf'], engine: 'mineru' }],
  },
  vlm_config: { enabled: true, model_id: 'vlm-1', description_language: 'Chinese', custom_instructions: 'describe' },
  asr_config: { enabled: false, model_id: '', language: '' },
  question_generation_config: { enabled: true, question_count: 5, custom_instructions: 'gen' },
  storage_backend_id: 'st-1',
  storage_provider_config: { provider: 's3' },
  extract_config: { enabled: true, text: 'people', tags: ['t1'], nodes: [{ name: 'A', attributes: ['x'] }], relations: [], custom_instructions: 'graph rules' },
};

test('config update path matches the Vue PUT /initialization/config endpoint', () => {
  assert.equal(getKnowledgeBaseConfigPath('kb/a'), '/api/v1/initialization/config/kb%2Fa');
});

test('payload mirrors the Vue KBModelConfigRequest update shape exactly', () => {
  const rules = [{ file_types: ['pdf'], engine: 'opendataloader' }];
  const payload = buildKnowledgeSettingsConfigPayload(knowledgeBase, rules, { enabled: true, text: 'people', tags: ['t1'], nodes: [{ name: 'A', attributes: ['x'] }], relations: [], customInstructions: 'graph rules' });
  assert.deepEqual(payload, {
    llmModelId: 'llm-1',
    embeddingModelId: 'embed-1',
    vlm_config: { enabled: true, model_id: 'vlm-1', description_language: 'Chinese', custom_instructions: 'describe' },
    asr_config: { enabled: false, model_id: '', language: '' },
    documentSplitting: {
      chunkSize: 700,
      chunkOverlap: 90,
      separators: ['\n\n', '\n'],
      parserEngineRules: rules,
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      strategy: 'auto',
      tokenLimit: 0,
      languages: ['zh'],
      tableMetadataInstructions: 'keep headers',
    },
    multimodal: { enabled: true },
    storageBackendId: 'st-1',
    storageProvider: 's3',
    nodeExtract: { enabled: true, text: 'people', tags: ['t1'], nodes: [{ name: 'A', attributes: ['x'] }], relations: [], customInstructions: 'graph rules' },
    questionGeneration: { enabled: true, questionCount: 5, customInstructions: 'gen' },
  });
});

test('payload fills Vue fallbacks for sparse knowledge bases and drops the immutable vector binding', () => {
  const payload = buildKnowledgeSettingsConfigPayload({ id: 'kb-2', name: 'Bare', type: 'document' }, [], undefined);
  assert.deepEqual(payload, {
    llmModelId: '',
    embeddingModelId: '',
    vlm_config: { enabled: false, model_id: '', description_language: '', custom_instructions: '' },
    asr_config: { enabled: false, model_id: '', language: '' },
    documentSplitting: {
      chunkSize: 512,
      chunkOverlap: 80,
      separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
      parserEngineRules: [],
      enableParentChild: false,
      parentChunkSize: 4096,
      childChunkSize: 384,
      strategy: '',
      tokenLimit: 0,
      languages: [],
      tableMetadataInstructions: '',
    },
    multimodal: { enabled: false },
    storageBackendId: '',
    storageProvider: 'local',
    nodeExtract: { enabled: false, text: '', tags: [], nodes: [], relations: [], customInstructions: '' },
    questionGeneration: { enabled: false, questionCount: 3, customInstructions: '' },
  });
  assert.equal('vector_store_id' in payload, false, 'vector-store binding is create-only in the Vue update contract');
});

test('saveKnowledgeSettings PUTs the payload through the authenticated client transport', async () => {
  const calls: Array<{ method: string; path: string; body: unknown }> = [];
  const client = {
    request: async (input: { method: string; path: string; body: unknown }) => {
      calls.push({ method: input.method, path: input.path, body: input.body });
      return { success: true };
    },
  } as unknown as WeKnoraClient;
  const payload = buildKnowledgeSettingsConfigPayload(knowledgeBase, [], undefined);
  await saveKnowledgeSettings(client, 'kb/1', payload);
  assert.deepEqual(calls, [{ method: 'PUT', path: '/api/v1/initialization/config/kb%2F1', body: payload }]);
});

interface UiCalls { requests: Array<{ method: string; path: string; body: Record<string, unknown> }> }

function documentsList(request: (input: { method: string; path: string; body: Record<string, unknown> }) => Promise<unknown>) {
  // Vue isIndexingLocked probe (loadKBData): routed through the same transport
  // as every other request so the counts include it.
  return async (kbId: string, params: Record<string, number> = {}) => {
    const query = new URLSearchParams(Object.entries(params).map(([key, value]) => [key, String(value)]));
    const suffix = query.toString();
    return await request({ method: 'GET', path: `/api/v1/knowledge-bases/${encodeURIComponent(kbId)}/knowledge${suffix ? `?${suffix}` : ''}`, body: {} });
  };
}

function clientFor(calls: UiCalls, mode: 'ok' | 'fail' = 'ok'): WeKnoraClient {
  const request = async (input: { method: string; path: string; body: Record<string, unknown> }) => {
    calls.requests.push({ method: input.method, path: input.path, body: input.body });
    if (mode === 'fail') throw new Error('storage backend unavailable');
    return { success: true };
  };
  return {
    request,
    knowledgeBases: {
      documents: { list: documentsList(request) },
      settings: {
        parserEngines: async () => ({ data: [{ Name: 'mineru', Description: 'MinerU', Available: true, FileTypes: ['pdf'] }, { Name: 'builtin', Description: 'Built-in', Available: true, FileTypes: ['pdf'] }] }),
        storageBackends: async () => ({ data: [{ id: 'st-1', name: 'Main storage', provider: 's3', status: 'ready' }] }),
        vectorStores: async () => ({ data: [{ id: 'vs-1', name: 'Vectors', engine_type: 'pgvector', source: 'tenant', readonly: false }] }),
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient, role: 'owner' | 'admin' | 'viewer'): Promise<HTMLElement> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase, role }));
  });
  await act(async () => { await Promise.resolve(); });
  return container;
}

function findButton(label: string): HTMLButtonElement {
  const button = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === label);
  assert.ok(button, `expected a "${label}" button; got: ${JSON.stringify([...document.body.querySelectorAll('button')].map((candidate) => candidate.textContent))}`);
  return button as HTMLButtonElement;
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

test('save button is gated to owner/admin and disabled while a save is in flight (no double submit)', async () => {
  const calls: UiCalls = { requests: [] };
  let releaseSave: (() => void) | undefined;
  const gate = new Promise<void>((resolve) => { releaseSave = resolve; });
  const slowClient = {
    request: async (input: { method: string; path: string; body: Record<string, unknown> }) => {
      calls.requests.push({ method: input.method, path: input.path, body: input.body });
      await gate;
      return { success: true };
    },
    knowledgeBases: {
      documents: { list: documentsList(async (input) => { calls.requests.push({ method: input.method, path: input.path, body: input.body }); await gate; return { success: true }; }) },
      settings: {
        parserEngines: async () => ({ data: [{ Name: 'mineru', Description: 'MinerU', Available: true }] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
      },
    },
  } as unknown as WeKnoraClient;
  await renderPage(slowClient, 'owner');
  const save = findButton('Save and Close');
  assert.equal(save.disabled, false, 'idle save button must be enabled for owner');
  await act(async () => {
    save.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
    await Promise.resolve();
  });
  assert.equal(calls.requests.length, 2, 'the mount-time documents probe plus exactly one in-flight request pair (base update)');
  assert.equal(save.disabled, true, 'save button must be disabled while saving (Vue :loading="saving")');
  await act(async () => {
    save.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
    await Promise.resolve();
  });
  assert.equal(calls.requests.length, 2, 'repeat clicks while saving must be ignored');
  await act(async () => { releaseSave!(); await gate; });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 3, 'mount probe, then the base update, then the config PUT');
  assert.equal(save.disabled, false, 'save button re-enables after the save settles');
  assert.match(document.body.textContent ?? '', /Configuration saved successfully/);
});

test('a successful save persists the Vue update payload carrying the pending parser engine', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), 'admin');
  await act(async () => {
    const sectionButton = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').includes('Parser'));
    sectionButton!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  const select = [...document.body.querySelectorAll('select')].find((candidate) => candidate.getAttribute('aria-label') === 'PDF Documents')!;
  assert.ok(select, 'expected the per-file-type parser engine select for the pdf group');
  assert.equal(select.value, 'mineru', 'the committed pdf rule preselects the group select');
  await act(async () => {
    [...select.options].forEach((option) => { option.selected = option.value === 'builtin'; });
    select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
  await act(async () => { findButton('Save and Close').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 3, 'mount probe plus the Vue doSubmit pair: base update first, then the config PUT');
  const request = calls.requests[2]!;
  assert.equal(request.method, 'PUT');
  assert.equal(request.path, '/api/v1/initialization/config/kb-1');
  const splitting = (request.body as { documentSplitting?: { parserEngineRules?: Array<Record<string, unknown>> } }).documentSplitting;
  assert.deepEqual(splitting?.parserEngineRules, [{ file_types: ['pdf'], engine: 'builtin' }], 'the pending parser engine must be persisted, matching the summary preview');
  assert.equal((request.body as { llmModelId?: string }).llmModelId, 'llm-1');
  assert.match(document.body.textContent ?? '', /Configuration saved successfully/);
});

test('a failed save keeps the form state and surfaces the error message', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, 'fail'), 'admin');
  await act(async () => {
    const sectionButton = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').includes('Parser'));
    sectionButton!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  const select = [...document.body.querySelectorAll('select')].find((candidate) => candidate.getAttribute('aria-label') === 'PDF Documents')!;
  assert.ok(select, 'expected the per-file-type parser engine select for the pdf group');
  await act(async () => {
    [...select.options].forEach((option) => { option.selected = option.value === 'builtin'; });
    select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
  await act(async () => { findButton('Save and Close').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.match(document.body.textContent ?? '', /storage backend unavailable/, 'the server error message must surface');
  assert.equal(document.body.textContent?.includes('Configuration saved successfully'), false, 'no success feedback on failure');
  const selectAfter = [...document.body.querySelectorAll('select')].find((candidate) => candidate.getAttribute('aria-label') === 'PDF Documents')!;
  assert.equal(selectAfter.value, 'builtin', 'the pending selection must survive a failed save');
  assert.equal(selectAfter.disabled, false, 'the form must stay editable after a failed save');
  await act(async () => { findButton('Save and Close').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 3, 'mount probe plus the original pair and the retry PUT');
});

test('viewers get no save button (Vue canManage gating)', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), 'viewer');
  const labels = [...document.body.querySelectorAll('button')].map((candidate) => (candidate.textContent ?? '').trim());
  assert.equal(labels.includes('Save and Close'), false, 'viewer must not see the save button');
  assert.equal(calls.requests.length, 1, 'only the mount-time documents probe runs for a viewer');
});

// Vue loadKBData seeds chunking with `||` fallbacks: a stored chunk_overlap 0
// (unset) reads as the 80 DefaultChunkOverlap, and an empty separators array
// (truthy) is kept as-is — both observed live on the Parity KB Demo fixture
// (R441 browser evidence: React showed overlap 0 where Vue showed 80).
test('seeds chunking values with the Vue || fallbacks for zero and empty-array stored configs', () => {
  const payload = buildKnowledgeSettingsConfigPayload({
    id: 'kb-3',
    name: 'Zeroed chunking',
    type: 'document',
    chunking_config: { chunk_overlap: 0, separators: [] },
  }, [], undefined);
  assert.equal(payload.documentSplitting.chunkOverlap, 80, 'stored 0 overlap reads as the Vue || fallback 80');
  assert.deepEqual(payload.documentSplitting.separators, [], 'an empty separators array is truthy in Vue and must be kept');
});
