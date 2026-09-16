// R442 A1 round: Vue-parity follow-ups on the knowledge settings surface.
// 1. Separators render as the Vue multiple-creatable tag chips (KBChunkingSettings.vue),
//    not a native listbox.
// 2. The Vue "测试分块效果" debug entry (KBChunkingDebug.vue) posts the live config
//    to /api/v1/chunker/preview and renders the tier/stats/chunks result.
// 3. Save validates the Vue models contract (validateForm): the Embedding model is
//    required only while RAG search (vector|keyword) is on, the summary model always.
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
  chunking_config: {
    chunk_size: 700,
    chunk_overlap: 90,
    separators: ['\n\n', '\n'],
    enable_parent_child: false,
    strategy: '',
    token_limit: 0,
    languages: [],
  },
  vlm_config: { enabled: false, model_id: '', description_language: '', custom_instructions: '' },
  asr_config: { enabled: false, model_id: '', language: '' },
};

interface UiCalls {
  requests: Array<{ method: string; path: string; body: Record<string, unknown> }>;
  previews: Array<{ text: string; chunking_config: Record<string, unknown> }>;
}

function clientFor(calls: UiCalls, options: { preview?: () => unknown } = {}): WeKnoraClient {
  return {
    request: async (input: { method: string; path: string; body: Record<string, unknown> }) => {
      calls.requests.push({ method: input.method, path: input.path, body: input.body });
      return { success: true };
    },
    configuration: {
      models: { list: async () => [] },
    },
    knowledgeBases: {
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
        previewChunking: async (input: { text: string; chunking_config: Record<string, unknown> }) => {
          calls.previews.push(input);
          return options.preview ? options.preview() : {
            selected_tier: 'heading',
            tier_chain: ['heading', 'heuristic', 'legacy'],
            rejected: [{ tier: 'heuristic', reason: 'no structure match' }],
            chunks: [{ seq: 1, start: 0, end: 120, size_chars: 120, size_tokens_approx: 40, context_header: 'Intro', content: 'hello chunk' }],
            stats: { count: 1, avg_chars: 120, min_chars: 120, max_chars: 120, stddev_chars: 0 },
            profile: { total_lines: 4, total_chars: 120, md_heading_total: 1, form_feed_count: 0, german_chapter_count: 0, english_chapter_count: 0, chinese_chapter_count: 0, detected_langs: ['en'] },
          };
        },
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient, kb: KnowledgeSettingsInput = knowledgeBase): Promise<void> {
  // A test that mounts several pages in sequence must not leave the previous
  // instance in the DOM (its save button would hijack later clicks).
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
  await act(async () => { await Promise.resolve(); });
}

async function openSection(section: string): Promise<void> {
  const button = document.body.querySelector(`button[data-section="${section}"]`);
  assert.ok(button, `expected a ${section} section button`);
  await act(async () => { button!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
}

function labeledControl(selector: string, label: string): HTMLElement {
  const control = [...document.body.querySelectorAll<HTMLElement>(selector)].find((candidate) => candidate.getAttribute('aria-label') === label);
  assert.ok(control, `expected a ${selector} with aria-label "${label}"; got: ${JSON.stringify([...document.body.querySelectorAll(selector)].map((candidate) => candidate.getAttribute('aria-label')))}`);
  return control;
}

function setNativeValue(element: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  const prototype = element instanceof dom.window.HTMLTextAreaElement ? dom.window.HTMLTextAreaElement.prototype : dom.window.HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(prototype, 'value')!.set!;
  setter.call(element, value);
  element.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  element.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

function pressKey(element: HTMLElement, key: string): void {
  element.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
}

function clickSave(): Promise<void> {
  const save = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === 'Save Configuration');
  assert.ok(save, 'expected the save button');
  return act(async () => { save!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
}

function configPuts(calls: UiCalls): Array<{ method: string; path: string; body: Record<string, unknown> }> {
  return calls.requests.filter((request) => request.path.startsWith('/api/v1/initialization/config/'));
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

test('separators render as the Vue creatable tag chips with add/remove instead of a listbox', async () => {
  const calls: UiCalls = { requests: [], previews: [] };
  await renderPage(clientFor(calls));
  await openSection('chunking');

  // The native multi-select listbox is gone; the control is the Vue chips field.
  const listbox = [...document.body.querySelectorAll('select')].find((candidate) => candidate.getAttribute('aria-label') === 'Separators');
  assert.ok(listbox === undefined, 'separators must not render as a native multi-select listbox anymore');

  const chips = [...document.body.querySelectorAll<HTMLElement>('[data-separator-chip]')];
  const chipText = chips.map((chip) => (chip.textContent ?? '').trim());
  assert.ok(chipText.some((value) => value.startsWith('Double newline')), `expected a chip for \\n\\n, got: ${JSON.stringify(chipText)}`);
  assert.ok(chipText.some((value) => value.startsWith('Single newline')), `expected a chip for \\n, got: ${JSON.stringify(chipText)}`);

  // Every chip carries its remove button (Vue t-select tag close icon).
  const removeButtons = chips.map((chip) => chip.querySelector('button'));
  assert.ok(removeButtons.every((button) => button !== null), 'each chip exposes a remove button');
  assert.ok(
    removeButtons.every((button) => (button!.getAttribute('aria-label') ?? '').startsWith('Remove')),
    'remove buttons carry an accessible Remove label',
  );

  // Focus opens the preset dropdown listing the not-yet-selected separators.
  const input = labeledControl('input', 'Separators') as HTMLInputElement;
  assert.equal(input.tagName, 'INPUT', 'the control offers a text input for custom separators');
  await act(async () => { input.focus(); });
  const optionText = () => [...document.body.querySelectorAll('[role="option"]')].map((option) => (option.textContent ?? '').trim());
  assert.ok(optionText().some((value) => value.startsWith('Chinese period')), `focused control lists the preset options, got: ${JSON.stringify(optionText())}`);

  // Clicking a preset adds it (Vue dropdown option toggle).
  const period = [...document.body.querySelectorAll('[role="option"]')].find((option) => (option.textContent ?? '').startsWith('Chinese period'));
  assert.ok(period);
  await act(async () => { period!.dispatchEvent(new dom.window.MouseEvent('mousedown', { bubbles: true, cancelable: true })); });
  const chipsAfterPreset = [...document.body.querySelectorAll<HTMLElement>('[data-separator-chip]')];
  assert.ok(chipsAfterPreset.some((chip) => (chip.textContent ?? '').startsWith('Chinese period')), 'the preset chip is added');

  // Typing a custom separator and pressing Enter commits it as a chip.
  await act(async () => { setNativeValue(input, '++'); });
  await act(async () => { pressKey(input, 'Enter'); });
  const chipsAfterCustom = [...document.body.querySelectorAll<HTMLElement>('[data-separator-chip]')];
  assert.ok(chipsAfterCustom.some((chip) => (chip.textContent ?? '').includes('++')), 'the typed custom separator becomes a chip');

  // Esc clears the pending draft without committing it.
  await act(async () => { setNativeValue(input, 'zz'); });
  await act(async () => { pressKey(input, 'Escape'); });
  assert.equal(input.value, '', 'Escape clears the pending draft');
  const chipsAfterEsc = [...document.body.querySelectorAll<HTMLElement>('[data-separator-chip]')];
  assert.ok(!chipsAfterEsc.some((chip) => (chip.textContent ?? '').includes('zz')), 'Escape does not commit the draft');

  // Removing a chip drops the value from the draft.
  const newlineChip = chipsAfterEsc.find((chip) => (chip.textContent ?? '').startsWith('Single newline'));
  assert.ok(newlineChip);
  const removeButton = newlineChip!.querySelector('button');
  assert.ok(removeButton);
  await act(async () => { removeButton!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  assert.ok(![...document.body.querySelectorAll('[data-separator-chip]')].some((chip) => (chip.textContent ?? '').startsWith('Single newline')), 'the chip disappears after its remove click');

  // The draft reaches the save payload in Vue order.
  await clickSave();
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const puts = configPuts(calls);
  assert.equal(puts.length, 1, 'one config PUT');
  const splitting = (puts[0]!.body as Record<string, any>).documentSplitting;
  assert.deepEqual(splitting.separators, ['\n\n', '。', '++'], 'chip edits flow into the save payload');
});

test('the Vue debug entry previews chunking through the chunker API and renders the result', async () => {
  const calls: UiCalls = { requests: [], previews: [] };
  await renderPage(clientFor(calls));
  await openSection('chunking');

  const trigger = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').includes('Preview chunking'));
  assert.ok(trigger, 'expected the Preview chunking trigger next to the strategy picker');

  await act(async () => { trigger!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  const sample = labeledControl('textarea', 'Sample text') as HTMLTextAreaElement;
  assert.ok(sample.value.length > 0, 'opening the drawer auto-loads the default preset sample');
  assert.equal(sample.maxLength, 64 * 1024, 'the sample textarea mirrors the backend previewMaxChars cap');

  const run = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === 'Run preview');
  assert.ok(run, 'expected the Run preview button');

  await act(async () => { run!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.previews.length, 1, 'one preview call');
  assert.equal(calls.previews[0]!.chunking_config.chunk_size, 700, 'the preview posts the live committed chunking config');
  assert.deepEqual(calls.previews[0]!.chunking_config.separators, ['\n\n', '\n']);

  assert.match(document.body.textContent ?? '', /Selected strategy/, 'the result header renders');
  assert.match(document.body.textContent ?? '', /1\s*chunks/, 'the chunk count renders');
  assert.match(document.body.textContent ?? '', /hello chunk/, 'the chunk content renders');
});

test('save enforces the Vue models contract: embedding required only with RAG on, summary always', async () => {
  // Missing Embedding while vector search is on → blocked with the Vue warning.
  const callsA: UiCalls = { requests: [], previews: [] };
  await renderPage(clientFor(callsA), { ...knowledgeBase, embedding_model_id: '' });
  await clickSave();
  await act(async () => { await Promise.resolve(); });
  assert.equal(callsA.requests.length, 0, 'Vue validateForm returns before any request');
  assert.match(document.body.textContent ?? '', /RAG search requires an Embedding model/);
  const activeA = document.body.querySelector('button[data-section="models"][aria-current="page"]');
  assert.ok(activeA, 'the editor jumps to the models section');

  // Missing summary model → blocked with the Vue warning, also jumping to models.
  const callsB: UiCalls = { requests: [], previews: [] };
  await renderPage(clientFor(callsB), { ...knowledgeBase, summary_model_id: '' });
  await clickSave();
  await act(async () => { await Promise.resolve(); });
  assert.equal(callsB.requests.length, 0);
  assert.match(document.body.textContent ?? '', /Please select a summary model/);
  assert.ok(document.body.querySelector('button[data-section="models"][aria-current="page"]'));

  // Wiki-only strategy (vector+keyword off): Embedding becomes optional and the
  // save proceeds once the summary model is present.
  const callsC: UiCalls = { requests: [], previews: [] };
  await renderPage(clientFor(callsC), {
    ...knowledgeBase,
    embedding_model_id: '',
    indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true, graph_enabled: false },
  });
  await clickSave();
  await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); });
  assert.equal(configPuts(callsC).length, 1, 'wiki-only strategies save without an embedding model');
  assert.match(document.body.textContent ?? '', /Configuration saved successfully/);
});
