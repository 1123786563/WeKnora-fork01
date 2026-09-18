// R445 A3 round: the two R444 signal leftovers, checked against the Vue
// sources (KnowledgeBaseEditorModal.vue + KBModelConfig.vue).
//
// 1. Storage select edit semantics (KBStorageSettings.vue): the select binds
//    `:disabled="!!props.hasFiles"` where the modal passes
//    `:has-files="editorMode === 'edit' && hasFiles"`. An edit-mode KB without
//    files keeps the instance selector editable, and handleChange emits BOTH
//    the backend id and the selected backend's provider, which persist through
//    the PUT initialization/config body (storage_backend_id + provider
//    projection) — the same pipeline the React save already owns.
//
// 2. Embedding row visibility (KBModelConfig.vue): the row binds
//    `v-if="ragEnabled !== false || wikiEnabled"`, so a pure-LLM KB (vector and
//    keyword indexing explicitly off, wiki off) hides the whole row, while a
//    wiki-only KB keeps it with the optional copy
//    (knowledgeEditor.models.embeddingWikiOptionalDesc) and no required mark.
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
const { KnowledgeSettingsPage } = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

const knowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
  storage_backend_id: 'st-1',
  storage_provider_config: { provider: 's3' },
  indexing_strategy: { vector_enabled: true, keyword_enabled: false, wiki_enabled: false, graph_enabled: false },
};

interface UiCalls {
  requests: Array<{ method: string; path: string; body: Record<string, unknown> }>;
}

function clientFor(calls: UiCalls, options: { documentsTotal?: number } = {}): WeKnoraClient {
  const request = async (input: { method: string; path: string; body: Record<string, unknown> }) => {
    calls.requests.push({ method: input.method, path: input.path, body: input.body });
    return { success: true };
  };
  return {
    request,
    configuration: {
      models: { list: async () => [] },
    },
    knowledgeBases: {
      documents: {
        list: async (kbId: string, params: Record<string, number> = {}) => {
          const query = new URLSearchParams(Object.entries(params).map(([key, value]) => [key, String(value)]));
          const suffix = query.toString();
          await request({ method: 'GET', path: `/api/v1/knowledge-bases/${encodeURIComponent(kbId)}/knowledge${suffix ? `?${suffix}` : ''}`, body: {} });
          return { data: [], total: options.documentsTotal ?? 0 };
        },
      },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [
          { id: 'st-1', name: 'Main storage', provider: 's3', status: 'ready' },
          { id: 'st-2', name: 'Backup storage', provider: 'oss', status: 'ready' },
        ] }),
        vectorStores: async () => ({ data: [] }),
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient, kb: KnowledgeSettingsInput = knowledgeBase): Promise<void> {
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
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase: kb, role: 'admin' }));
  });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

async function openSection(key: string): Promise<void> {
  const button = document.body.querySelector(`button[data-section="${key}"]`);
  assert.ok(button, `expected a ${key} section button`);
  await act(async () => { button!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); });
}

function storageSelect(): HTMLSelectElement {
  const select = [...document.body.querySelectorAll<HTMLSelectElement>('select[aria-label]')]
    .find((candidate) => /storage instance/i.test(candidate.getAttribute('aria-label') ?? ''));
  assert.ok(select, `expected the storage instance select; got: ${JSON.stringify([...document.body.querySelectorAll('select')].map((candidate) => candidate.getAttribute('aria-label')))}`);
  return select!;
}

function embeddingSelect(): HTMLSelectElement | undefined {
  return [...document.body.querySelectorAll<HTMLSelectElement>('select[aria-label]')]
    .find((candidate) => /Embedding/i.test(candidate.getAttribute('aria-label') ?? ''));
}

async function saveConfiguration(calls: UiCalls): Promise<Record<string, unknown>> {
  const save = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === 'Save Configuration');
  assert.ok(save, 'expected the save button');
  await act(async () => { save!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); });
  const configPut = calls.requests.find((request) => request.method === 'PUT' && request.path === '/api/v1/initialization/config/kb-1');
  assert.ok(configPut, 'expected the PUT initialization/config request');
  return configPut!.body as Record<string, unknown>;
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

test('an edit-mode KB without files keeps the storage instance selector editable', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 0 }));
  await openSection('storage');
  assert.equal(storageSelect().disabled, false, 'Vue `:disabled="!!props.hasFiles"` is false without files');
});

test('an edit-mode KB with files keeps the storage instance selector locked', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 7 }));
  await openSection('storage');
  assert.equal(storageSelect().disabled, true, 'Vue `:disabled="!!props.hasFiles"` is true with files');
});

test('picking another storage instance persists the backend id and its provider through the config PUT', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 0 }));
  await openSection('storage');
  const select = storageSelect();
  await act(async () => {
    select.value = 'st-2';
    select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
  const body = await saveConfiguration(calls);
  assert.equal(body.storageBackendId, 'st-2', 'the selected backend id must reach the PUT body');
  assert.equal(body.storageProvider, 'oss', 'the selected backend provider must reach the PUT body (Vue handleChange emits both)');
});

test('a pure-LLM KB hides the whole Embedding row (Vue v-if ragEnabled !== false || wikiEnabled)', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 7 }), {
    ...knowledgeBase,
    indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: false, graph_enabled: false },
  });
  await openSection('models');
  assert.equal(embeddingSelect(), undefined, 'the Embedding row is removed, not merely disabled');
  const llm = [...document.body.querySelectorAll<HTMLSelectElement>('select[aria-label]')].find((candidate) => /LLM/i.test(candidate.getAttribute('aria-label') ?? ''));
  assert.ok(llm, 'the LLM row stays visible');
});

test('a wiki-only KB keeps the Embedding row optional and editable despite files', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 7 }), {
    ...knowledgeBase,
    indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true, graph_enabled: false },
  });
  await openSection('models');
  const embedding = embeddingSelect();
  assert.ok(embedding, 'the Embedding row stays visible for wiki classification');
  assert.equal(embedding!.disabled, false, 'the wiki-only Embedding selector stays editable (ragEnabled false)');
  const label = [...document.body.querySelectorAll('label')].find((candidate) => /Embedding/i.test(candidate.textContent ?? ''));
  assert.ok(label, 'expected the Embedding row label');
  assert.equal((label!.textContent ?? '').includes('*'), false, 'the wiki-only Embedding row drops the required mark');
  const description = label!.parentElement?.querySelector('p');
  assert.match(description?.textContent ?? '', /Wiki directory classification/, 'the wiki-optional description replaces the RAG copy');
});
