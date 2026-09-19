// R441 A1: basic section migration (Vue KnowledgeBaseEditorModal.vue basic
// section) plus the document-KB base-update wiring (Vue doSubmit step 1 —
// updateKnowledgeBase carries name/description and, for document bases,
// wiki_config + auto_tag_config + indexing_strategy before the config PUT).
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
const { KnowledgeSettingsPage, buildKnowledgeSettingsBaseUpdate, getKnowledgeBaseUpdatePath } = await import('./KnowledgeSettingsPage.tsx');
const { createTranslator } = await import('../i18n.ts');
const t = createTranslator('en-US');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;
type KnowledgeSettingsEditorOverrides = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsEditorOverrides;

// Document KB carrying the wiki/auto-tag/indexing round-trip state the Vue
// editor loads (loadKBData contract).
const documentKb: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  description: 'D',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
  wiki_config: { synthesis_model_id: 'wiki-llm', max_pages_per_ingest: 2, extraction_granularity: 'focused', content_instructions: 'cite sources', extraction_instructions: 'products' },
  indexing_strategy: { vector_enabled: true, keyword_enabled: false, wiki_enabled: false, graph_enabled: false },
  auto_tag_config: { enabled: true, model_id: 'tag-llm', max_tags: 5, skip_if_tagged: false },
};

const faqKb: KnowledgeSettingsInput = {
  id: 'kb-faq',
  name: 'Support FAQ',
  description: 'Answers',
  type: 'faq',
  faq_config: { index_mode: 'question_only', question_index_mode: 'separate' },
};

// ---- Base-update payload contract (Vue doSubmit → updateKnowledgeBase) ----

test('document base update round-trips wiki/auto-tag/indexing strategy through the Vue updateConfig shape', () => {
  assert.equal(getKnowledgeBaseUpdatePath('kb/1'), '/api/v1/knowledge-bases/kb%2F1');
  const baseUpdate = buildKnowledgeSettingsBaseUpdate(documentKb);
  assert.deepEqual(baseUpdate, {
    name: 'Product docs',
    description: 'D',
    config: {
      wiki_config: { synthesis_model_id: 'wiki-llm', max_pages_per_ingest: 2, extraction_granularity: 'focused', content_instructions: 'cite sources', extraction_instructions: 'products' },
      auto_tag_config: { enabled: true, model_id: 'tag-llm', max_tags: 5, skip_if_tagged: false },
      indexing_strategy: { vector_enabled: true, keyword_enabled: false, wiki_enabled: false, graph_enabled: false },
    },
  });
});

test('sparse document base update fills the Vue defaults', () => {
  const baseUpdate = buildKnowledgeSettingsBaseUpdate({ id: 'kb-2', name: 'Bare', type: 'document' });
  assert.deepEqual(baseUpdate, {
    name: 'Bare',
    description: '',
    config: {
      wiki_config: { synthesis_model_id: '', max_pages_per_ingest: 0, extraction_granularity: 'standard', content_instructions: '', extraction_instructions: '' },
      auto_tag_config: { enabled: false, model_id: '', max_tags: 3, skip_if_tagged: true },
      indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: false },
    },
  });
});

test('basic overrides replace only the touched fields and keep the rest of the round-trip', () => {
  const baseUpdate = buildKnowledgeSettingsBaseUpdate(documentKb, {
    name: 'Renamed docs',
    description: 'New description',
    indexing: { wikiEnabled: true },
    wiki: { extractionGranularity: 'exhaustive', contentInstructions: 'tone: legal' },
  } satisfies KnowledgeSettingsEditorOverrides);
  assert.equal(baseUpdate.name, 'Renamed docs');
  assert.equal(baseUpdate.description, 'New description');
  assert.deepEqual(baseUpdate.config.indexing_strategy, { vector_enabled: true, keyword_enabled: false, wiki_enabled: true, graph_enabled: false }, 'untouched indexing flags keep the round-trip values');
  assert.equal(baseUpdate.config.wiki_config!.extraction_granularity, 'exhaustive');
  assert.equal(baseUpdate.config.wiki_config!.content_instructions, 'tone: legal');
  assert.equal(baseUpdate.config.wiki_config!.extraction_instructions, 'products', 'untouched wiki fields keep the round-trip values');
  assert.equal(baseUpdate.config.wiki_config!.synthesis_model_id, 'wiki-llm');
  assert.deepEqual(baseUpdate.config.auto_tag_config, { enabled: true, model_id: 'tag-llm', max_tags: 5, skip_if_tagged: false }, 'auto-tag config round-trips untouched');
});

test('FAQ base update never carries the document-only config blocks', () => {
  const baseUpdate = buildKnowledgeSettingsBaseUpdate(faqKb);
  assert.deepEqual(Object.keys(baseUpdate.config).sort(), ['faq_config'], 'Vue attaches only faq_config for FAQ bases');
});

// ---- Rendered basic section ----

interface UiCalls { requests: Array<{ method: string; path: string; body: Record<string, unknown> }> }

function clientFor(calls: UiCalls): WeKnoraClient {
  const request = async (input: { method: string; path: string; body: Record<string, unknown> }) => {
    calls.requests.push({ method: input.method, path: input.path, body: input.body });
    return { success: true };
  };
  return {
    request,
    knowledgeBases: {
      documents: {
        // Vue isIndexingLocked probe (loadKBData): routed through the same
        // transport as every other request so the counts include it.
        list: async (kbId: string, params: Record<string, number> = {}) => {
          const query = new URLSearchParams(Object.entries(params).map(([key, value]) => [key, String(value)]));
          const suffix = query.toString();
          await request({ method: 'GET', path: `/api/v1/knowledge-bases/${encodeURIComponent(kbId)}/knowledge${suffix ? `?${suffix}` : ''}`, body: {} });
          return { data: [], total: 0 };
        },
      },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
      },
    },
    configuration: { models: { list: async () => [] } },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient = clientFor({ requests: [] }), knowledgeBase: KnowledgeSettingsInput = documentKb): Promise<void> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase, role: 'owner' }));
  });
  await act(async () => { await Promise.resolve(); });
}

async function openSection(section: string): Promise<void> {
  const button = document.body.querySelector(`button[data-section="${section}"]`);
  assert.ok(button, `expected a ${section} nav item; got: ${JSON.stringify([...document.body.querySelectorAll('button[data-section]')].map((candidate) => candidate.getAttribute('data-section')))}`);
  await act(async () => { button!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); });
}

function labeled<T extends Element>(selector: string, label: string): T {
  const translated = t(label);
  const element = [...document.body.querySelectorAll(selector)].find((candidate) => candidate.getAttribute('aria-label') === translated) as T | undefined;
  assert.ok(element, `expected ${selector} labelled "${label}" (${translated}); got: ${JSON.stringify([...document.body.querySelectorAll(selector)].map((candidate) => candidate.getAttribute('aria-label')))}`);
  return element;
}

async function setValue(element: HTMLInputElement | HTMLTextAreaElement, value: string, checked?: boolean): Promise<void> {
  await act(async () => {
    if (element instanceof dom.window.HTMLInputElement && (element.type === 'checkbox' || element.type === 'radio')) {
      if (element.type === 'checkbox') element.checked = !(checked ?? !element.checked);
      element.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
    } else {
      const prototype = element instanceof dom.window.HTMLTextAreaElement ? dom.window.HTMLTextAreaElement.prototype : dom.window.HTMLInputElement.prototype;
      const setter = Object.getOwnPropertyDescriptor(prototype, 'value')!.set!;
      setter.call(element, value);
      element.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
      element.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
    }
  });
  await act(async () => { await Promise.resolve(); });
}

async function clickSave(): Promise<void> {
  const save = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === 'Save and Close');
  assert.ok(save, 'expected the save button');
  await act(async () => { save!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

test('basic section renders the Vue kb id, type radios, name and description editors', async () => {
  await renderPage();
  await openSection('basic');

  // Vue edit-mode kb-id field with the committed identifier.
  const kbId = [...document.body.querySelectorAll('code')].find((node) => node.textContent === 'kb-1');
  assert.ok(kbId, 'the committed knowledge-base id must render (Vue kb-id-field)');

  // Type radio group is disabled on the edit surface (Vue :disabled="editorMode === 'edit'").
  const documentRadio = labeled<HTMLInputElement>('input[type="radio"]', 'knowledgeEditor.basic.typeDocument');
  const faqRadio = labeled<HTMLInputElement>('input[type="radio"]', 'knowledgeEditor.basic.typeFAQ');
  assert.equal(documentRadio.checked, true);
  assert.equal(faqRadio.checked, false);
  assert.equal(documentRadio.disabled, true, 'the type is immutable on edit (Vue disabled radio group)');

  const name = labeled<HTMLInputElement>('input', 'knowledgeEditor.basic.nameLabel') as HTMLInputElement;
  assert.equal(name.value, 'Product docs', 'the committed name preselects the editor');
  assert.equal(name.maxLength, 50, 'Vue t-input :maxlength="50"');
  const description = labeled<HTMLTextAreaElement>('textarea', 'knowledgeEditor.basic.descriptionLabel');
  assert.equal(description.value, 'D');
  assert.equal(description.maxLength, 200, 'Vue t-textarea :maxlength="200"');

  // Vue basic section carries the indexing-strategy checks for document bases.
  const rag = labeled<HTMLInputElement>('input[type="checkbox"]', 'knowledgeEditor.indexing.searchTitle') as HTMLInputElement;
  const wiki = labeled<HTMLInputElement>('input[type="checkbox"]', 'knowledgeEditor.indexing.wikiTitle') as HTMLInputElement;
  assert.equal(rag.checked, true, 'committed vector_enabled');
  assert.equal(wiki.checked, false, 'committed wiki_enabled');

  // The wiki subsection stays hidden while the wiki strategy is off (Vue v-if).
  assert.ok(!labeledSafe('input[type="radio"]', 'knowledgeEditor.wiki.granularityStandard'), 'granularity radios hide until wiki is enabled');
  assert.ok(!labeledSafe('textarea', 'knowledgeEditor.wiki.contentInstructionsLabel'), 'content instructions hide until wiki is enabled');
});

function labeledSafe(selector: string, label: string): boolean {
  const translated = t(label);
  return [...document.body.querySelectorAll(selector)].some((candidate) => candidate.getAttribute('aria-label') === translated);
}

test('enabling the wiki strategy reveals the Vue granularity radios, hint and instruction textareas', async () => {
  await renderPage();
  await openSection('basic');
  const wiki = labeled<HTMLInputElement>('input[type="checkbox"]', 'knowledgeEditor.indexing.wikiTitle') as HTMLInputElement;
  await setValue(wiki, '', true);

  const focused = labeled<HTMLInputElement>('input[type="radio"]', 'knowledgeEditor.wiki.granularityFocused') as HTMLInputElement;
  assert.equal(focused.checked, true, 'committed extraction_granularity preselects the radio');
  assert.ok(labeledSafe('input[type="radio"]', 'knowledgeEditor.wiki.granularityStandard'));
  assert.ok(labeledSafe('input[type="radio"]', 'knowledgeEditor.wiki.granularityExhaustive'));
  assert.match(document.body.textContent ?? '', /main subjects/, 'the Vue granularity hint renders under the radios');

  const content = labeled<HTMLTextAreaElement>('textarea', 'knowledgeEditor.wiki.contentInstructionsLabel');
  assert.equal(content.value, 'cite sources');
  assert.equal(content.maxLength, 4000, 'Vue t-textarea :maxlength="4000"');
  const extraction = labeled<HTMLTextAreaElement>('textarea', 'knowledgeEditor.wiki.extractionInstructionsLabel');
  assert.equal(extraction.value, 'products');
  assert.equal(extraction.maxLength, 4000);
});

test('empty name blocks the save with the Vue nameRequired toast before any request', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('basic');
  const name = labeled<HTMLInputElement>('input', 'knowledgeEditor.basic.nameLabel') as HTMLInputElement;
  await setValue(name, '   ');
  await clickSave();
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 1, 'Vue validateForm returns before any request (only the mount-time documents probe ran)');
  assert.match(document.body.textContent ?? '', /Please enter the knowledge base name/);
  assert.equal(document.body.querySelector('button[data-section="basic"]')?.className.includes('is-active'), true, 'Vue jumps back to the basic section');
});

test('disabling every indexing strategy blocks the save with the Vue atLeastOne toast', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('basic');
  const rag = labeled<HTMLInputElement>('input[type="checkbox"]', 'knowledgeEditor.indexing.searchTitle') as HTMLInputElement;
  const wiki = labeled<HTMLInputElement>('input[type="checkbox"]', 'knowledgeEditor.indexing.wikiTitle') as HTMLInputElement;
  await setValue(rag, '', false);
  await setValue(wiki, '', false);
  await clickSave();
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 1, 'Vue validateForm returns before any request (only the mount-time documents probe ran)');
  assert.match(document.body.textContent ?? '', /At least one indexing strategy must be enabled/);
  assert.equal(document.body.querySelector('button[data-section="basic"]')?.className.includes('is-active'), true);
});

test('document save issues the Vue doSubmit pair: base update first, then the config PUT', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('basic');
  const name = labeled<HTMLInputElement>('input', 'knowledgeEditor.basic.nameLabel') as HTMLInputElement;
  const wiki = labeled<HTMLInputElement>('input[type="checkbox"]', 'knowledgeEditor.indexing.wikiTitle') as HTMLInputElement;
  await setValue(name, 'Renamed docs');
  await setValue(wiki, '', true);
  await clickSave();
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });

  assert.equal(calls.requests.length, 3, 'the mount-time documents probe plus the Vue doSubmit pair: base update first, then the config PUT');
  const base = calls.requests[1]!;
  assert.equal(base.method, 'PUT');
  assert.equal(base.path, '/api/v1/knowledge-bases/kb-1');
  assert.deepEqual(base.body, {
    name: 'Renamed docs',
    description: 'D',
    config: {
      wiki_config: { synthesis_model_id: 'wiki-llm', max_pages_per_ingest: 2, extraction_granularity: 'focused', content_instructions: 'cite sources', extraction_instructions: 'products' },
      auto_tag_config: { enabled: true, model_id: 'tag-llm', max_tags: 5, skip_if_tagged: false },
      indexing_strategy: { vector_enabled: true, keyword_enabled: false, wiki_enabled: true, graph_enabled: false },
    },
  });
  const configPut = calls.requests[2]!;
  assert.equal(configPut.path, '/api/v1/initialization/config/kb-1');
  assert.equal('name' in configPut.body, false, 'the KBModelConfigRequest body carries no name (matches Vue)');
  assert.match(document.body.textContent ?? '', /Configuration saved successfully/);
});
