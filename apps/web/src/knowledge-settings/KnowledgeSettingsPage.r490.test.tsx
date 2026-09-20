// R490 A: the four substantive R489 residuals on the KB settings page
// (.wkbs-modal / /knowledgeBase/:id/settings) — all text parity against the
// Vue baseline container (.settings-modal):
// #10 models section — Vue KBModelConfig.vue renders the llmDesc /
//     embeddingDesc help lines under the selectors; React shipped the labels
//     without them, so the section body text diverged.
// #13 chunking section — Vue KBChunkingSettings.vue carries a per-field desc
//     (sizeDescription/overlapDescription/…) plus t-slider marks
//     (100/1000/2000/4000, 0/250/500, 512/2048/4096/8192, 64/384/1024/2048);
//     the shared React chunking form shipped bare sliders.
// #18 storage section — Vue KBStorageSettings.vue renders ONE bound-instance
//     select with the endpoint hint and the 管理存储实例 entry; React rendered
//     an instance list instead.
// #7 tag manage entry lives in documents/tag-manage-interaction.test.tsx.
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

function clientFor(): WeKnoraClient {
  return {
    request: async () => ({ success: true }),
    configuration: {
      models: { list: async () => [] },
    },
    knowledgeBases: {
      documents: {
        list: async () => ({ data: [], total: 0 }),
      },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [], default_storage_backend_id: '' }),
        vectorStores: async () => ({ data: [] }),
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(kb: KnowledgeSettingsInput = knowledgeBase): Promise<void> {
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
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client: clientFor(), knowledgeBase: kb, role: 'admin' }));
  });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

async function openSection(key: string): Promise<void> {
  const button = document.body.querySelector(`button[data-section="${key}"]`);
  assert.ok(button, `expected a ${key} section button`);
  await act(async () => { button!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); });
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

// #10 — Vue KBModelConfig.vue setting-info block: label + <p class="desc">
// (llmDesc / embeddingDesc). R489 saw the two desc lines as the whole V>-only
// diff on this section.
test('#10 the models section renders the Vue llmDesc and embeddingDesc help lines', async () => {
  await renderPage();
  await openSection('models');
  const text = document.body.textContent;

  // en-US: "Large language model used for summarization and abstract generation (optional)"
  assert.match(text, /Large language model used for summarization/, 'the Vue knowledgeEditor.models.llmDesc renders under the LLM label');
  // en-US: "Embedding model used for text vectorization"
  assert.match(text, /Embedding model used for text vectorization/, 'the Vue knowledgeEditor.models.embeddingDesc renders under the Embedding label');
});

// #13 — Vue KBChunkingSettings.vue: every setting-row carries a desc line and
// every t-slider renders its marks under the track.
test('#13 the chunking section renders the Vue per-field desc help lines', async () => {
  await renderPage();
  await openSection('chunking');
  const text = document.body.textContent;

  // en-US values from knowledgeEditorMessages — assert a stable prefix of each.
  assert.match(text, /Maximum characters per chunk/, 'sizeDescription (chunk size help)');
  assert.match(text, /characters shared between adjacent chunks/i, 'overlapDescription (chunk overlap help)');
  assert.match(text, /Parent-child chunking|two-level chunking/i, 'parentChildDescription (parent-child help)');
  // Separators help (Vue separatorsDescription).
  assert.match(text, /Characters or strings the splitter prefers/i, 'separatorsDescription (separator help)');
});

test('#13 the chunking sliders render the Vue numeric marks tiers', async () => {
  await renderPage();
  await openSection('chunking');
  const body = document.body;

  // Vue chunkSizeMarks {100,1000,2000,4000} and chunkOverlapMarks {0,250,500}
  // render as tick labels under the two always-visible sliders.
  const sizeSlider = body.querySelector('input[type="range"][aria-label*="chunk size" i], input[type="range"][min="100"][max="4000"]');
  assert.ok(sizeSlider, 'expected the chunk-size range input');
  const sizeMarks = sizeSlider!.parentElement!.querySelector('[data-slider-marks]');
  assert.ok(sizeMarks, 'the chunk-size slider renders a marks tier row (Vue t-slider marks)');
  const sizeMarksText = (sizeMarks! as HTMLElement).textContent;
  for (const mark of ['100', '1000', '2000', '4000']) {
    assert.ok(sizeMarksText.includes(mark), `chunk-size marks include ${mark}`);
  }

  const overlapSlider = body.querySelector('input[type="range"][min="0"][max="500"]');
  assert.ok(overlapSlider, 'expected the chunk-overlap range input');
  const overlapMarks = overlapSlider!.parentElement!.querySelector('[data-slider-marks]');
  assert.ok(overlapMarks, 'the chunk-overlap slider renders a marks tier row');
  const overlapMarksText = (overlapMarks! as HTMLElement).textContent;
  for (const mark of ['0', '250', '500']) {
    assert.ok(overlapMarksText.includes(mark), `chunk-overlap marks include ${mark}`);
  }
});

test('#13 the parent-child sliders render the Vue marks tiers once enabled', async () => {
  await renderPage();
  await openSection('chunking');

  // Toggle the parent-child switch on (Vue v-if="localEnableParentChild").
  const toggle = document.body.querySelector('input[type="checkbox"][aria-label*="parent" i]');
  assert.ok(toggle, 'expected the parent-child toggle');
  await act(async () => { toggle!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); });

  const parentSlider = document.body.querySelector('input[type="range"][min="512"][max="8192"]');
  assert.ok(parentSlider, 'expected the parent chunk-size range input');
  const parentMarks = (parentSlider!.parentElement!.querySelector('[data-slider-marks]') as HTMLElement | null);
  assert.ok(parentMarks, 'the parent slider renders a marks tier row');
  for (const mark of ['512', '2048', '4096', '8192']) {
    assert.ok(parentMarks!.textContent.includes(mark), `parent marks include ${mark}`);
  }

  const childSlider = document.body.querySelector('input[type="range"][min="64"][max="2048"]');
  assert.ok(childSlider, 'expected the child chunk-size range input');
  const childMarks = (childSlider!.parentElement!.querySelector('[data-slider-marks]') as HTMLElement | null);
  assert.ok(childMarks, 'the child slider renders a marks tier row');
  for (const mark of ['64', '384', '1024', '2048']) {
    assert.ok(childMarks!.textContent.includes(mark), `child marks include ${mark}`);
  }
});

// #18 — Vue KBStorageSettings.vue: ONE bound-instance select + hint + the
// manage-instances entry. The R489 residual was React rendering the raw
// instance rows ("Parity COS · COS"…) as a list.
test('#18 the storage section renders the Vue bound-select form with the manage entry, not an instance list', async () => {
  await renderPage();
  await openSection('storage');
  const body = document.body;

  // The single bound-instance select (aria-label = 存储实例 / Storage instance).
  const select = body.querySelector('select[aria-label]');
  assert.ok(select, 'expected the storage instance select');

  // Vue renders the 管理存储实例 entry as an anchor that jumps to the
  // tenant storage settings.
  const manage = body.querySelector('[data-storage-manage-instances]');
  assert.ok(manage, 'the manage-instances entry renders');
  assert.match((manage! as HTMLElement).textContent, /Manage|管理/, 'the manage entry carries the kbSettings.storage.manageInstances copy');
});
