// R444 A1 round: the Vue Embedding lock contract (KBModelConfig.vue rendered
// from KnowledgeBaseEditorModal.vue `currentSection === 'models'`). The same
// R443 probe signal (edit-mode listKnowledgeFiles page_size=1, total > 0)
// drives a second surface: with the RAG retrieval enabled
// (vectorEnabled || keywordEnabled) and the KB already holding files, the
// Embedding selector disables and the Vue knowledgeEditor.models.embeddingLocked
// warning renders in the info column. With every index strategy off the Vue
// binding `ragEnabled && hasFiles` evaluates false, so the selector stays
// editable even though the basic indexing checks remain locked. A failed probe
// degrades to unlocked. The storage section (KBStorageSettings.vue) renders its
// kbSettings.storage.migrateHint only while the KB has files.
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
  indexing_strategy: { vector_enabled: true, keyword_enabled: false, wiki_enabled: false, graph_enabled: false },
};

interface UiCalls {
  requests: Array<{ method: string; path: string; body: Record<string, unknown> }>;
}

function clientFor(calls: UiCalls, options: { documentsTotal?: number; documentsError?: Error } = {}): WeKnoraClient {
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
          if (options.documentsError) throw options.documentsError;
          return { data: [], total: options.documentsTotal ?? 0 };
        },
      },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
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

function modelSelectors(): { llm: HTMLSelectElement; embedding: HTMLSelectElement } {
  const selects = [...document.body.querySelectorAll<HTMLSelectElement>('select[aria-label]')];
  const embedding = selects.find((candidate) => /Embedding/i.test(candidate.getAttribute('aria-label') ?? ''));
  const llm = selects.find((candidate) => candidate !== embedding);
  assert.ok(embedding, 'expected the Embedding model select');
  assert.ok(llm, 'expected the LLM model select');
  return { llm: llm!, embedding: embedding! };
}

function embeddingLockedTip(): string | null {
  const tip = document.body.querySelector('[data-embedding-locked-tip]');
  return tip ? (tip.textContent ?? '').trim() : null;
}

function storageMigrateHint(): string | null {
  const hint = document.body.querySelector('[data-storage-migrate-hint]');
  return hint ? (hint.textContent ?? '').trim() : null;
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

test('a KB with files locks the Embedding selector with the Vue warning while RAG stays on', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 7 }));
  await openSection('models');

  const { llm, embedding } = modelSelectors();
  assert.equal(embedding.disabled, true, 'the Embedding selector disables when ragEnabled && hasFiles');
  assert.equal(llm.disabled, false, 'the LLM selector never locks on this signal');
  assert.match(embeddingLockedTip() ?? '', /already has files/, 'the Vue knowledgeEditor.models.embeddingLocked warning renders in the info column');

  // The locked draft still saves the committed embedding model — the lock is
  // a UI contract, not a save blocker.
  const save = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === 'Save Configuration');
  assert.ok(save);
  await act(async () => { save!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); });
  const configPut = calls.requests.find((request) => request.method === 'PUT' && request.path === '/api/v1/initialization/config/kb-1');
  assert.ok(configPut, 'the locked models section still saves');
  assert.equal((configPut!.body as Record<string, unknown>).embeddingModelId, 'embed-1', 'the save payload keeps the committed embedding model');
});

test('with every index strategy off the Embedding selector stays editable despite files', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 7 }), {
    ...knowledgeBase,
    indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: false, graph_enabled: false },
  });
  await openSection('models');

  const { embedding } = modelSelectors();
  assert.equal(embedding.disabled, false, 'the Vue binding ragEnabled && hasFiles is false without vector/keyword indexing');
  assert.equal(embeddingLockedTip(), null, 'no Embedding warning renders without RAG retrieval');
});

test('an empty KB keeps the Embedding selector editable and hides the warning', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 0 }));
  await openSection('models');

  const { embedding } = modelSelectors();
  assert.equal(embedding.disabled, false, 'an empty KB keeps the Embedding selector editable');
  assert.equal(embeddingLockedTip(), null, 'no Embedding warning renders without files');
});

test('a failed documents probe degrades to an unlocked Embedding selector', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsError: new Error('probe rejected') }));
  await openSection('models');

  const { embedding } = modelSelectors();
  assert.equal(embedding.disabled, false, 'a failed probe must not lock the Embedding selector');
  assert.equal(embeddingLockedTip(), null, 'no Embedding warning after a failed probe');
});

test('the storage migrate hint renders only while the KB has files', async () => {
  const withFiles: UiCalls = { requests: [] };
  await renderPage(clientFor(withFiles, { documentsTotal: 7 }));
  await openSection('storage');
  assert.match(storageMigrateHint() ?? '', /storage migration flow/, 'the Vue kbSettings.storage.migrateHint renders with files');

  const empty: UiCalls = { requests: [] };
  await renderPage(clientFor(empty, { documentsTotal: 0 }));
  await openSection('storage');
  assert.equal(storageMigrateHint(), null, 'no migrate hint without files');
});
