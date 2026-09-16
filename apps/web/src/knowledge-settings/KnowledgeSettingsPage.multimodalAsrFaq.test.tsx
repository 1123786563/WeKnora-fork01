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
const { KnowledgeSettingsPage, buildKnowledgeSettingsConfigPayload, buildKnowledgeSettingsBaseUpdate, getKnowledgeBaseUpdatePath } = await import('./KnowledgeSettingsPage.tsx');
const { createTranslator } = await import('../i18n.ts');
const t = createTranslator('en-US');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;
type KnowledgeSettingsEditorOverrides = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsEditorOverrides;

// Document KB with multimodal/asr round-trip state (Vue loadKBData contract).
const documentKb: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
  vlm_config: { enabled: true, model_id: 'vlm-1', description_language: 'Chinese', custom_instructions: 'describe' },
  asr_config: { enabled: true, model_id: 'asr-1', language: 'zh-CN' },
};

// FAQ KB as returned by GET /knowledge-bases/:id (faq_config round-trip).
const faqKb: KnowledgeSettingsInput = {
  id: 'kb-faq',
  name: 'Support FAQ',
  description: 'Answers',
  type: 'faq',
  summary_model_id: 'llm-1',
  // Vue validateForm requires the Embedding model while RAG search (vector or
  // keyword indexing) is enabled — FAQ bases default to both on.
  embedding_model_id: 'embed-1',
  faq_config: { index_mode: 'question_only', question_index_mode: 'separate' },
};

const catalogue = [
  { id: 'llm-1', name: 'LLM 1', display_name: 'LLM 1', type: 'KnowledgeQA', source: 'system', status: 'active' },
  { id: 'vlm-1', name: 'VLM 1', display_name: 'VLM 1', type: 'VLLM', source: 'system', status: 'active' },
  { id: 'vlm-9', name: 'VLM 9', display_name: 'VLM 9', type: 'VLLM', source: 'system', status: 'active' },
  { id: 'vlm-2', name: 'VLM 2', display_name: 'VLM 2', type: 'VLLM', source: 'system', status: 'active' },
  { id: 'asr-1', name: 'ASR 1', display_name: 'ASR 1', type: 'ASR', source: 'system', status: 'active' },
  { id: 'asr-9', name: 'ASR 9', display_name: 'ASR 9', type: 'ASR', source: 'system', status: 'active' },
  { id: 'asr-2', name: 'ASR 2', display_name: 'ASR 2', type: 'ASR', source: 'system', status: 'active' },
];

interface UiCalls { requests: Array<{ method: string; path: string; body: Record<string, unknown> }> }

function clientFor(calls: UiCalls, mode: 'ok' | 'fail' = 'ok'): WeKnoraClient {
  return {
    request: async (input: { method: string; path: string; body: Record<string, unknown> }) => {
      calls.requests.push({ method: input.method, path: input.path, body: input.body });
      if (mode === 'fail') throw new Error('save rejected');
      return { success: true };
    },
    knowledgeBases: {
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
      },
    },
    configuration: { models: { list: async () => catalogue } },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient, knowledgeBase: KnowledgeSettingsInput, role: 'owner' | 'admin' | 'viewer' = 'owner'): Promise<HTMLElement> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase, role }));
  });
  // Flush the Promise.allSettled catalogue loads (editorResources contract).
  for (let i = 0; i < 4; i += 1) {
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  }
  return container;
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

async function setValue(element: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement, value: string, checked?: boolean): Promise<void> {
  await act(async () => {
    if (element instanceof dom.window.HTMLInputElement && element.type === 'checkbox') {
      // jsdom click activation toggles `checked` before listeners run, so
      // pre-set the opposite of the desired state and let the click flip it.
      element.checked = !(checked ?? !element.checked);
      element.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
    } else if (element instanceof dom.window.HTMLInputElement && element.type === 'radio') {
      // Click the unchecked radio; jsdom activation checks it before React's
      // listener runs, which is what drives onChange.
      element.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
    } else if (element instanceof dom.window.HTMLSelectElement) {
      [...element.options].forEach((option) => { option.selected = option.value === value; });
      element.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
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

function navButton(label: string): HTMLButtonElement {
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

// ---- payload contract (Vue buildSubmitData vlm_config / asr_config) ----

test('multimodal overrides drive vlm_config with the Vue disabled-clears-model rule', () => {
  const enabled = buildKnowledgeSettingsConfigPayload(
    { ...documentKb, vlm_config: { enabled: false, model_id: '', description_language: 'Chinese', custom_instructions: 'describe' } },
    [],
    undefined,
    { multimodal: { enabled: true, vllmModelId: 'vlm-9', descriptionLanguage: 'Korean', customInstructions: 'alt text' } },
  );
  assert.deepEqual(enabled.vlm_config, { enabled: true, model_id: 'vlm-9', description_language: 'Korean', custom_instructions: 'alt text' });
  assert.deepEqual(enabled.multimodal, { enabled: true });

  // Vue handleMultimodalToggle clears vllmModelId when switched off; the
  // payload keeps description_language/custom_instructions round-trip values.
  const disabled = buildKnowledgeSettingsConfigPayload(documentKb, [], undefined, { multimodal: { enabled: false } });
  assert.deepEqual(disabled.vlm_config, { enabled: false, model_id: '', description_language: 'Chinese', custom_instructions: 'describe' });
  assert.deepEqual(disabled.multimodal, { enabled: false });
});

test('asr overrides drive asr_config with the Vue disabled-clears-model rule', () => {
  const enabled = buildKnowledgeSettingsConfigPayload(
    { ...documentKb, asr_config: { enabled: false, model_id: '', language: 'zh-CN' } },
    [],
    undefined,
    { asr: { enabled: true, modelId: 'asr-9' } },
  );
  assert.deepEqual(enabled.asr_config, { enabled: true, model_id: 'asr-9', language: 'zh-CN' }, 'untouched language keeps the round-trip value');

  const disabled = buildKnowledgeSettingsConfigPayload(documentKb, [], undefined, { asr: { enabled: false } });
  assert.deepEqual(disabled.asr_config, { enabled: false, model_id: '', language: 'zh-CN' });
});

// ---- FAQ base-update contract (Vue doSubmit → updateKnowledgeBase) ----

test('FAQ save carries faq_config through the base knowledge-base update, not the config PUT', () => {
  assert.equal(getKnowledgeBaseUpdatePath('kb/faq'), '/api/v1/knowledge-bases/kb%2Ffaq');

  const roundTrip = buildKnowledgeSettingsBaseUpdate(faqKb);
  assert.deepEqual(roundTrip, { name: 'Support FAQ', description: 'Answers', config: { faq_config: { index_mode: 'question_only', question_index_mode: 'separate' } } });

  const overridden = buildKnowledgeSettingsBaseUpdate(faqKb, { faqConfig: { indexMode: 'question_answer', questionIndexMode: 'combined' } });
  assert.deepEqual(overridden.config.faq_config, { index_mode: 'question_answer', question_index_mode: 'combined' });

  // Vue only attaches faq_config for FAQ type knowledge bases.
  const documentUpdate = buildKnowledgeSettingsBaseUpdate(documentKb);
  assert.equal('faq_config' in documentUpdate.config, false, 'document bases must not carry faq_config');
});

// ---- multimodal section UI ----

test('multimodal section renders the Vue rows and saves the draft through the PUT pipeline', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), documentKb);
  await openSection('multimodal');
  assert.equal((document.body.textContent ?? '').includes('not been ported'), false, 'multimodal must no longer render the placeholder');

  const toggle = labeled<HTMLInputElement>('input[type="checkbox"]', 'knowledgeEditor.advanced.multimodal.label');
  assert.equal(toggle.checked, true, 'toggle reflects the round-trip vlm_config.enabled');

  const vllm = labeled<HTMLSelectElement>('select', 'knowledgeEditor.advanced.multimodal.vllmLabel');
  assert.equal(vllm.value, 'vlm-1');
  assert.deepEqual([...vllm.options].map((option) => option.value), ['', 'vlm-1', 'vlm-9', 'vlm-2'], 'only available VLLM models are offered (Vue ModelSelector)');

  const language = labeled<HTMLSelectElement>('select', 'knowledgeEditor.advanced.multimodal.descriptionLanguageLabel');
  assert.deepEqual([...language.options].map((option) => option.value), ['', 'Chinese', 'English', 'Korean', 'Russian'], 'Vue languageOptions with the clearable auto placeholder');

  const instructions = labeled<HTMLTextAreaElement>('textarea', 'knowledgeEditor.advanced.multimodal.customInstructionsLabel');
  assert.equal(instructions.maxLength, 4000, 'Vue t-textarea :maxlength="4000"');
  assert.equal(instructions.value, 'describe');

  await setValue(vllm, 'vlm-9');
  await setValue(language, 'Korean');
  await setValue(instructions, 'alt text');
  await act(async () => { navButton('Save Configuration').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 2, 'R441: Vue doSubmit now also runs the base update for document bases');
  const body = calls.requests[1]!.body as { vlm_config?: Record<string, unknown> };
  assert.deepEqual(body.vlm_config, { enabled: true, model_id: 'vlm-9', description_language: 'Korean', custom_instructions: 'alt text' });
});

test('switching multimodal off hides the Vue conditional rows and clears model_id on save', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), documentKb);
  await openSection('multimodal');
  const toggle = labeled<HTMLInputElement>('input[type="checkbox"]', 'knowledgeEditor.advanced.multimodal.label');
  await setValue(toggle, '', false);
  assert.equal(document.body.querySelector(`select[aria-label="${t('knowledgeEditor.advanced.multimodal.vllmLabel')}"]`), null, 'Vue v-if="multimodalConfig.enabled" hides the VLLM row');

  await act(async () => { navButton('Save Configuration').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const body = calls.requests[1]!.body as { vlm_config?: Record<string, unknown> };
  assert.deepEqual(body.vlm_config, { enabled: false, model_id: '', description_language: 'Chinese', custom_instructions: 'describe' });
});

test('enabled multimodal without a model blocks the save with the Vue multimodalInvalid toast', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), { ...documentKb, vlm_config: { enabled: true, model_id: '', description_language: '', custom_instructions: '' } });
  await openSection('multimodal');
  await act(async () => { navButton('Save Configuration').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 0, 'Vue validateForm returns before any request');
  assert.match(document.body.textContent ?? '', /multimodal/i);
  assert.equal(document.body.querySelector('button[data-section="multimodal"]')?.className.includes('is-active'), true, 'Vue jumps to the multimodal section');
});

// ---- asr section UI ----

test('asr section renders the Vue toggle and ASR model selector', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), documentKb);
  await openSection('asr');
  assert.equal((document.body.textContent ?? '').includes('not been ported'), false, 'asr must no longer render the placeholder');

  const toggle = labeled<HTMLInputElement>('input[type="checkbox"]', 'knowledgeEditor.asr.label');
  assert.equal(toggle.checked, true);

  const model = labeled<HTMLSelectElement>('select', 'knowledgeEditor.asr.modelLabel');
  assert.equal(model.value, 'asr-1');
  assert.deepEqual([...model.options].map((option) => option.value), ['', 'asr-1', 'asr-9', 'asr-2'], 'only available ASR models are offered');

  await setValue(model, 'asr-9');
  await act(async () => { navButton('Save Configuration').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const body = calls.requests[1]!.body as { asr_config?: Record<string, unknown> };
  assert.deepEqual(body.asr_config, { enabled: true, model_id: 'asr-9', language: 'zh-CN' });
});

// ---- faq section UI ----

test('faq section renders the Vue index-mode radios and saves through the base update first', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), faqKb);
  await openSection('faq');
  assert.equal((document.body.textContent ?? '').includes('not been ported'), false, 'faq must no longer render the placeholder');

  const questionOnly = labeled<HTMLInputElement>('input[type="radio"]', 'knowledgeEditor.faq.modes.questionOnly');
  const questionAnswer = labeled<HTMLInputElement>('input[type="radio"]', 'knowledgeEditor.faq.modes.questionAnswer');
  const combined = labeled<HTMLInputElement>('input[type="radio"]', 'knowledgeEditor.faq.modes.combined');
  const separate = labeled<HTMLInputElement>('input[type="radio"]', 'knowledgeEditor.faq.modes.separate');
  assert.equal(questionOnly.checked, true, 'round-trip index_mode');
  assert.equal(separate.checked, true, 'round-trip question_index_mode');
  assert.equal(document.body.querySelector(`input[aria-label="${t('knowledgeEditor.faq.entryGuide')}"]`), null, 'entryGuide is guide copy, not a control');

  await setValue(questionAnswer, 'question_answer');
  assert.equal(combined.checked, false);
  await setValue(combined, 'combined');

  await act(async () => { navButton('Save Configuration').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.requests.length, 2, 'Vue doSubmit: base update first, then the config PUT');
  const base = calls.requests[0]!;
  assert.equal(base.method, 'PUT');
  assert.equal(base.path, '/api/v1/knowledge-bases/kb-faq');
  assert.deepEqual(base.body.config, { faq_config: { index_mode: 'question_answer', question_index_mode: 'combined' } });
  const configPut = calls.requests[1]!;
  assert.equal(configPut.path, '/api/v1/initialization/config/kb-faq');
  assert.equal('faq_config' in configPut.body, false, 'the KBModelConfigRequest body carries no faq_config (matches Vue)');
});

test('faq nav exposes the faq section only for FAQ bases and multimodal/asr stay document-only', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), faqKb);
  const faqKeys = [...document.body.querySelectorAll('button[data-section]')].map((button) => button.getAttribute('data-section'));
  assert.equal(faqKeys.includes('faq'), true, 'FAQ bases get the faq section (Vue navItems)');
  assert.equal(faqKeys.includes('multimodal'), false, 'Vue v-if="!isFAQ" gates multimodal away from FAQ bases');
  assert.equal(faqKeys.includes('asr'), false, 'Vue v-if="!isFAQ" gates asr away from FAQ bases');
});
