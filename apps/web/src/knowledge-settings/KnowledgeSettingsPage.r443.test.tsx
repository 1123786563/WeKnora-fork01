// R443 A1 round: the Vue isIndexingLocked contract (KnowledgeBaseEditorModal.vue).
// When the settings surface opens in edit mode for a document base the editor
// probes GET /api/v1/knowledge-bases/:id/knowledge once with page=1&page_size=1
// (Vue loadKBData listKnowledgeFiles). total > 0 means the KB already has
// content: the indexing strategy checks lock (the backend requires a non-empty
// KB to keep at least one index) and the Vue knowledgeEditor.indexing.lockedTip
// renders below the checks. A failed probe degrades to unlocked.
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

const faqKnowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-faq',
  name: 'FAQ base',
  type: 'faq',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
  faq_config: { index_mode: 'question_only', question_index_mode: 'separate' },
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
        // Mirrors the real transport: createKnowledgeDocumentsApi.list routes
        // through client.request with the page/page_size query string.
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

function indexingControls(): { rag: HTMLInputElement; wiki: HTMLInputElement } {
  const byLabel = (label: string): HTMLInputElement => {
    const control = [...document.body.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')].find((candidate) => candidate.getAttribute('aria-label') === label);
    assert.ok(control, `expected the ${label} indexing checkbox`);
    return control;
  };
  return { rag: byLabel('RAG Search'), wiki: byLabel('Wiki Knowledge Base') };
}

function lockedTip(): string | null {
  const tip = document.body.querySelector('[data-indexing-locked-tip]');
  return tip ? (tip.textContent ?? '').trim() : null;
}

function documentsProbes(calls: UiCalls): Array<{ method: string; path: string }> {
  return calls.requests.filter((request) => request.path.includes('/knowledge?')).map((request) => ({ method: request.method, path: request.path }));
}

async function clickCheckbox(control: HTMLInputElement): Promise<void> {
  await act(async () => { control.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })); });
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

test('edit-mode document KB with files locks the indexing checks with the Vue lockedTip', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 7 }));
  await openBasic();

  // The probe went out once, shaped like the Vue listKnowledgeFiles call.
  const probes = documentsProbes(calls);
  assert.deepEqual(probes, [{ method: 'GET', path: '/api/v1/knowledge-bases/kb-1/knowledge?page=1&page_size=1' }]);

  const { rag, wiki } = indexingControls();
  assert.equal(rag.disabled, true, 'the RAG search check locks when the KB has files');
  assert.equal(wiki.disabled, true, 'the wiki check locks when the KB has files');
  assert.match(lockedTip() ?? '', /cannot be changed once the knowledge base contains content/, 'the Vue knowledgeEditor.indexing.lockedTip renders below the checks');

  // Vue toggleVectorIndexing/toggleWikiIndexing early-return while locked, so
  // even a synthetic activation cannot flip the committed strategy.
  await clickCheckbox(rag);
  await clickCheckbox(wiki);
  assert.equal(rag.checked, true, 'the locked RAG check keeps its committed state');
  assert.equal(wiki.checked, false, 'the locked wiki check keeps its committed state');
  assert.equal(documentsProbes(calls).length, 1, 'the probe fires once per settings-surface open, never per interaction');

  // The locked draft still saves with the committed strategy (lock is a UI
  // contract, not a save blocker) — the base update keeps the round-trip flags.
  const save = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === 'Save and Close');
  assert.ok(save);
  await act(async () => { save!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); });
  const basePut = calls.requests.find((request) => request.method === 'PUT' && request.path === '/api/v1/knowledge-bases/kb-1');
  assert.ok(basePut, 'the locked surface still saves');
  assert.deepEqual((basePut!.body as Record<string, any>).config.indexing_strategy, { vector_enabled: true, keyword_enabled: false, wiki_enabled: false, graph_enabled: false }, 'the save payload keeps the committed indexing strategy');
});

async function openBasic(): Promise<void> {
  const button = document.body.querySelector('button[data-section="basic"]');
  assert.ok(button, 'expected a basic section button');
  await act(async () => { button!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); });
}

test('total=0 leaves the indexing checks editable and hides the locked tip', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsTotal: 0 }));
  await openBasic();

  assert.deepEqual(documentsProbes(calls), [{ method: 'GET', path: '/api/v1/knowledge-bases/kb-1/knowledge?page=1&page_size=1' }], 'the probe still runs for an empty base');
  const { rag, wiki } = indexingControls();
  assert.equal(rag.disabled, false, 'an empty KB keeps the RAG check editable');
  assert.equal(wiki.disabled, false, 'an empty KB keeps the wiki check editable');
  assert.equal(lockedTip(), null, 'no locked tip renders without files');

  await clickCheckbox(wiki);
  assert.equal(wiki.checked, true, 'the wiki check toggles on an empty KB');

  // A create-mode surface (no KB id) never probes at all.
  const createCalls: UiCalls = { requests: [] };
  await renderPage(clientFor(createCalls), { ...knowledgeBase, id: '' });
  await openBasic();
  assert.deepEqual(documentsProbes(createCalls), [], 'no probe without a KB id (Vue editorMode create)');
});

test('a failed documents probe degrades to unlocked instead of blocking the editor', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { documentsError: new Error('probe rejected') }));
  await openBasic();

  assert.equal(documentsProbes(calls).length, 1, 'the probe was attempted');
  const { rag, wiki } = indexingControls();
  assert.equal(rag.disabled, false, 'a failed probe must not lock the checks');
  assert.equal(wiki.disabled, false, 'a failed probe must not lock the checks');
  assert.equal(lockedTip(), null, 'no locked tip after a failed probe');
});

test('FAQ bases never probe the documents list (document-only signal)', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), faqKnowledgeBase);
  assert.deepEqual(documentsProbes(calls), [], 'the documents probe is document-type only');
});
