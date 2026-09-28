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
  // 分块段换 tdesign 控件（Select/Slider/Switch/InputNumber，弹层 Popup 系
  // 需要 Element/Node/SVGElement/rAF —— SandboxSettingsPanel.test 同款先例）。
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
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
} = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

// Mirrors the knowledge-base row the Vue editor loads for an existing KB.
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
  },
  question_generation_config: { enabled: true, question_count: 5, custom_instructions: 'gen rules' },
  vlm_config: { enabled: false, model_id: '', description_language: '', custom_instructions: '' },
  asr_config: { enabled: false, model_id: '', language: '' },
};

const liveModels = [
  { id: 'llm-1', name: 'gpt-x', display_name: 'GPT X', type: 'KnowledgeQA', source: 'remote' },
  { id: 'llm-2', name: 'qwen', type: 'KnowledgeQA', source: 'local' },
  { id: 'llm-dead', name: 'dead-llm', display_name: 'Dead LLM', type: 'KnowledgeQA', source: 'local', status: 'unavailable' },
  { id: 'embed-1', name: 'bge-m3', display_name: 'BGE M3', type: 'Embedding', source: 'local' },
  { id: 'embed-dead', name: 'dead-embed', type: 'Embedding', source: 'local', status: 'unavailable' },
  { id: 'rerank-1', name: 'rr', type: 'Rerank', source: 'local' },
];

interface UiCalls { requests: Array<{ method: string; path: string; body: Record<string, unknown> }> }

function clientFor(calls: UiCalls): WeKnoraClient {
  const request = async (input: { method: string; path: string; body: Record<string, unknown> }) => {
    calls.requests.push({ method: input.method, path: input.path, body: input.body });
    return { success: true };
  };
  return {
    request,
    configuration: {
      models: { list: async () => liveModels },
    },
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
        parserEngines: async () => ({ data: [{ Name: 'mineru', Description: 'MinerU', Available: true, FileTypes: ['pdf'] }] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient): Promise<void> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase, role: 'admin' }));
  });
  await act(async () => { await Promise.resolve(); });
}

async function openSection(section: string): Promise<void> {
  const button = document.body.querySelector(`button[data-section="${section}"]`);
  assert.ok(button, `expected a ${section} section button; got: ${JSON.stringify([...document.body.querySelectorAll('button[data-section]')].map((candidate) => candidate.textContent))}`);
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

function setSelectValues(select: HTMLSelectElement, values: string[]): void {
  [...select.options].forEach((option) => { option.selected = values.includes(option.value); });
  select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

function clickSave(): Promise<void> {
  const save = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === 'Save and Close');
  assert.ok(save, 'expected the save button');
  return act(async () => { save!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

test('payload overrides extend the round-trip payload only where editors touched values', () => {
  const untouched = buildKnowledgeSettingsConfigPayload(knowledgeBase, [], undefined);
  const overridden = buildKnowledgeSettingsConfigPayload(knowledgeBase, [], undefined, {
    llmModelId: 'llm-2',
    documentSplitting: { chunkSize: 800, chunkOverlap: 400 },
    questionGeneration: { questionCount: 7 },
  });
  assert.equal(overridden.llmModelId, 'llm-2');
  assert.equal(overridden.embeddingModelId, 'embed-1', 'fields without overrides keep the round-trip value');
  assert.equal(overridden.documentSplitting.chunkSize, 800);
  assert.equal(overridden.documentSplitting.chunkOverlap, 400);
  assert.equal(overridden.documentSplitting.strategy, 'auto', 'sibling chunking fields keep the round-trip value');
  assert.deepEqual(overridden.documentSplitting.separators, ['\n\n', '\n']);
  assert.equal(overridden.questionGeneration.questionCount, 7);
  assert.equal(overridden.questionGeneration.enabled, true, 'sibling advanced fields keep the round-trip value');
  assert.equal(overridden.questionGeneration.customInstructions, 'gen rules');
  assert.deepEqual(untouched.questionGeneration, { enabled: true, questionCount: 5, customInstructions: 'gen rules' }, 'without overrides the payload stays the exact R437 round-trip');
});

test('models section renders live llm/embedding selectors from the settings catalogue and saves the pending model ids', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('models');

  const llmSelect = labeledControl('select', 'LLM Model') as HTMLSelectElement;
  const embeddingSelect = labeledControl('select', 'Embedding Model') as HTMLSelectElement;
  assert.equal(llmSelect.value, 'llm-1', 'the committed summary model must preselect the LLM selector');
  assert.equal(embeddingSelect.value, 'embed-1');
  const llmValues = [...llmSelect.options].map((option) => option.value);
  assert.deepEqual(llmValues, ['', 'llm-1', 'llm-2'], 'options come from the live catalogue; unavailable models are excluded');
  const embeddingValues = [...embeddingSelect.options].map((option) => option.value);
  assert.deepEqual(embeddingValues, ['', 'embed-1'], 'embedding selector lists only available Embedding models');
  assert.equal([...llmSelect.options].find((option) => option.value === 'llm-2')!.textContent, 'qwen', 'display name falls back to the model name');

  await act(async () => { setSelectValues(llmSelect, ['llm-2']); });
  await clickSave();
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 3, 'the mount-time documents probe plus the doSubmit pair: base update before the config PUT for document bases');
  assert.equal(calls.requests[2]!.path, '/api/v1/initialization/config/kb-1');
  const savedBody = calls.requests[2]!.body as Record<string, any>;
  assert.equal(savedBody.llmModelId, 'llm-2', 'the pending LLM model must reach the PUT payload');
  assert.equal(savedBody.embeddingModelId, 'embed-1');
  assert.match(document.body.textContent ?? '', /Configuration saved successfully/);
});

test('chunking section renders the Vue tdesign chunking form and saves edited chunking values', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('chunking');

  /* px2-kb-settings-*：分块段换 KBChunkingSettings.vue 同构（.kb-chunking-
     settings 布局类族 + tdesign 控件），断言走 t-* DOM（SandboxSettingsPanel
     .test 同款口径）。 */
  const section = document.body.querySelector('.kb-chunking-settings');
  assert.ok(section, 'the chunking section renders the Vue .kb-chunking-settings shell');
  assert.match(section.querySelector('.section-header h2')?.textContent ?? '', /Chunking Settings/, 'the Vue section-header h2 rides inside the section component');

  // Strategy select (Vue t-select): the trigger shows the committed label.
  const strategyWrap = section.querySelector('.strategy-control .t-select__wrap');
  assert.ok(strategyWrap, 'the strategy control is a tdesign select');
  assert.match((strategyWrap.querySelector('input') as HTMLInputElement)?.value ?? '', /Automatic/, 'the committed strategy label shows in the trigger');

  // Sliders (Vue t-slider): two rows always render (size/overlap) plus the
  // parent/child pair while parent-child is on; marks render under the track.
  const sliderMarks = () => [...section.querySelectorAll('.t-slider')].map((slider) => [...slider.querySelectorAll('.t-slider__mark-text')].map((mark) => mark.textContent));
  assert.deepEqual(sliderMarks(), [
    ['100', '1000', '2000', '4000'],
    ['0', '250', '500'],
    ['512', '2048', '4096', '8192'],
    ['64', '384', '1024', '2048'],
  ], 'size/overlap + parent/child sliders render the Vue numeric mark tiers');
  const valueDisplays = [...section.querySelectorAll('.value-display')].map((node) => node.textContent);
  assert.deepEqual(valueDisplays, ['700 characters', '90 characters', '4096 characters', '384 characters'], 'the live value displays ride beside each slider');

  assert.equal((section.textContent ?? '').includes('Overlap is large compared to chunk size'), false, 'no warning while overlap is below half the chunk size');

  // Parent-child switch (Vue t-switch) is on; the t-is-checked state class
  // mirrors the committed flag (台账 #2 button-root 差异扫描豁免口径)。
  const parentSwitch = section.querySelector('.setting-row--toggle .t-switch') as HTMLElement;
  assert.ok(parentSwitch, 'the parent-child toggle renders as a tdesign switch');
  assert.match(parentSwitch.className, /t-is-checked/, 'the committed parent-child flag preselects the switch');

  // Advanced options collapsed by default (Vue v-if).
  const advancedToggle = [...section.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').includes('Advanced options'));
  assert.ok(advancedToggle, 'expected the collapsed advanced-options toggle');
  assert.ok(!section.querySelector('.advanced-section'), 'the advanced panel stays hidden until the toggle opens');
  await act(async () => { advancedToggle!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  const tokenInput = section.querySelector('.advanced-section .t-input-number input') as HTMLInputElement;
  assert.ok(tokenInput, 'the token limit renders as a tdesign input-number');
  assert.equal(tokenInput.value, '0');

  // Language hints select (multiple) renders its popup options on open.
  // tdesign 弹层触发器在内层 .t-input（点 wrap 不开弹层，jsdom 实证）。
  const languageInput = section.querySelector('.advanced-section .t-select__wrap .t-input') as HTMLElement;
  assert.ok(languageInput, 'the language hints control is a tdesign multiple select');
  await act(async () => { languageInput.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  const languageOptions = [...document.body.querySelectorAll('.t-select-option')].map((option) => (option.textContent ?? '').trim());
  assert.ok(languageOptions.some((value) => /German/.test(value)) && languageOptions.some((value) => /Chinese/.test(value)), `the language popup lists the Vue options, got: ${JSON.stringify(languageOptions)}`);

  // Strategy re-pick through the popup (SandboxSettingsPanel chooseBackend 同款；
  // 触发器在内层 .t-input).
  const strategyInput = strategyWrap.querySelector('.t-input') as HTMLElement;
  await act(async () => { strategyInput.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  const headingOption = [...document.body.querySelectorAll('.t-select-option')].find((option) => (option.textContent ?? '').trim() === 'Heading-aware');
  assert.ok(headingOption, 'the strategy popup lists the Vue options');
  await act(async () => { headingOption!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });

  await clickSave();
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 3, 'mount probe + base update + config PUT');
  const splitting = (calls.requests[2]!.body as Record<string, any>).documentSplitting;
  assert.equal(splitting.chunkSize, 700);
  assert.equal(splitting.chunkOverlap, 90);
  assert.equal(splitting.strategy, 'heading', 'the popup re-pick reaches the save payload');
  assert.equal(splitting.enableParentChild, true);
  assert.equal(splitting.parentChunkSize, 4096);
  assert.equal(splitting.childChunkSize, 384);
  assert.deepEqual(splitting.languages, ['zh']);
  assert.deepEqual(splitting.separators, ['\n\n', '\n'], 'untouched chunking fields keep the round-trip values');
  assert.match(document.body.textContent ?? '', /Configuration saved successfully/);
});

function labeledControlSafe(selector: string, label: string): boolean {
  return [...document.body.querySelectorAll<HTMLElement>(selector)].some((candidate) => candidate.getAttribute('aria-label') === label);
}

test('advanced section renders the question generation switch and persists edited question generation config', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('advanced');

  const toggle = labeledControl('input', 'AI Question Generation') as HTMLInputElement;
  assert.equal(toggle.type, 'checkbox');
  assert.equal(toggle.checked, true, 'the committed question generation flag preselects the switch');

  const count = labeledControl('input', 'Question Count') as HTMLInputElement;
  assert.equal(count.type, 'number');
  assert.equal(count.min, '1');
  assert.equal(count.max, '10');
  assert.equal(count.value, '5');

  const instructions = labeledControl('textarea', 'Question Generation Instructions') as HTMLTextAreaElement;
  assert.equal(instructions.maxLength, 4000);
  assert.equal(instructions.value, 'gen rules');

  // Toggle off: the Vue subsection collapses and the save must carry enabled=false.
  // jsdom checkbox activation toggles `checked` before the click listeners run,
  // so the click alone drives React's onChange.
  await act(async () => {
    toggle.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  assert.ok(!labeledControlSafe('input', 'Question Count'), 'count input collapses with the switch');

  await clickSave();
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 3, 'mount probe + base update + config PUT');
  assert.deepEqual((calls.requests[2]!.body as Record<string, any>).questionGeneration, { enabled: false, questionCount: 5, customInstructions: 'gen rules' }, 'the switch state must persist through the existing PUT pipeline');
  assert.match(document.body.textContent ?? '', /Configuration saved successfully/);

  // Re-enable and edit the count: the subsection re-opens with committed values.
  await act(async () => {
    toggle.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  const countAgain = labeledControl('input', 'Question Count') as HTMLInputElement;
  await act(async () => { setNativeValue(countAgain, '7'); });
  await clickSave();
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 5, 'mount probe plus two doSubmit pairs: base update + config PUT each');
  const retriedBody = calls.requests[4]!.body as Record<string, any>;
  assert.equal(retriedBody.questionGeneration.enabled, true);
  assert.equal(retriedBody.questionGeneration.questionCount, 7, 'the edited count must reach the PUT payload');
});
