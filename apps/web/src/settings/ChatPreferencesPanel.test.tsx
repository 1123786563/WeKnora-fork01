import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings?section=chat-preferences' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { ChatPreferencesPanel, chatDefaultModelOptions, defaultModelPreferencePatch, readDefaultModelPreference } = await import('./ChatPreferencesPanel.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.clear();
});

// SP14 Task 4 — the panel reads/writes the user default model through
// client.settings.preferences (PUT /auth/me/preferences, T3 merge semantics:
// default_model '' clears) and reuses the standard ModelOptionSelect dropdown
// with the chat KnowledgeQA filter.

test('readDefaultModelPreference echoes a trimmed default_model and guards shapes', () => {
  assert.equal(readDefaultModelPreference({ default_model: '  glm-5.3  ' }), 'glm-5.3');
  assert.equal(readDefaultModelPreference({ default_model: '' }), '');
  assert.equal(readDefaultModelPreference({}), '');
  assert.equal(readDefaultModelPreference({ default_model: 42 }), '');
  assert.equal(readDefaultModelPreference(null), '');
  assert.equal(readDefaultModelPreference('glm-5.3'), '');
  assert.equal(readDefaultModelPreference(undefined), '');
});

test('defaultModelPreferencePatch trims and keeps the empty-string clear payload', () => {
  assert.deepEqual(defaultModelPreferencePatch('  glm-5.3 '), { default_model: 'glm-5.3' });
  // T3 backend merge: default_model '' clears the preference.
  assert.deepEqual(defaultModelPreferencePatch(''), { default_model: '' });
});

test('chatDefaultModelOptions keeps KnowledgeQA rows with the display_name fallback', () => {
  const models = [
    { id: 'first-model', name: 'first-model', type: 'KnowledgeQA' },
    { id: 'glm-5.3', name: 'glm-5.3', display_name: 'GLM 5.3', type: 'KnowledgeQA' },
    { id: 'embed-1', name: 'embed-1', type: 'BGE-M3' },
    { id: 'rerank-1', name: 'rerank-1', type: 'BGE-RERANKER-V2-M3' },
  ];
  assert.deepEqual(chatDefaultModelOptions(models), [
    { value: 'first-model', label: 'first-model' },
    { value: 'glm-5.3', label: 'GLM 5.3' },
  ]);
});

interface PanelTestHarness {
  client: Parameters<typeof ChatPreferencesPanel>[0]['client'];
  updates: Array<Record<string, unknown>>;
}

function makeClient(options: { initialDefaultModel?: string; failUpdate?: boolean } = {}): PanelTestHarness {
  const updates: Array<Record<string, unknown>> = [];
  const client = {
    settings: {
      preferences: {
        get: async () => ({ default_model: options.initialDefaultModel ?? '' }),
        update: async (input: Record<string, unknown>) => {
          if (options.failUpdate) throw new Error('preferences offline');
          updates.push(input);
          return input;
        },
      },
    },
    configuration: {
      models: {
        list: async () => [
          { id: 'first-model', name: 'first-model', type: 'KnowledgeQA' },
          { id: 'glm-5.3', name: 'glm-5.3', display_name: 'GLM 5.3', type: 'KnowledgeQA' },
          { id: 'embed-1', name: 'embed-1', type: 'BGE-M3' },
        ],
      },
    },
  } as unknown as PanelTestHarness['client'];
  return { client, updates };
}

async function mountPanel(harness: PanelTestHarness, initialPreferences: unknown = { default_model: 'glm-5.3' }) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<ChatPreferencesPanel client={harness.client} initialPreferences={initialPreferences} />);
  });
  // Let the self-pulled model list land.
  await act(async () => { await Promise.resolve(); });
  return container;
}

test('panel echoes the saved default model and lists chat models only', async () => {
  const harness = makeClient();
  const container = await mountPanel(harness);

  assert.ok(container.querySelector('[data-testid="chat-preferences-panel"]'), 'the panel mounts');
  const combobox = container.querySelector<HTMLButtonElement>('[role="combobox"]');
  assert.ok(combobox, 'the default-model dropdown mounts');
  assert.equal(combobox.getAttribute('data-value'), 'glm-5.3', 'the stored default echoes in the closed trigger');
  assert.ok(container.textContent?.includes('默认对话模型'), 'the row label renders');
  assert.ok(container.textContent?.includes('会话内手动选择'), 'the priority hint renders');

  await act(async () => combobox.click());
  const optionValues = Array.from(container.querySelectorAll<HTMLButtonElement>('[role="option"]')).map((option) => option.getAttribute('data-value') ?? '');
  assert.deepEqual(optionValues, ['', 'first-model', 'glm-5.3'], 'the clear entry plus KnowledgeQA models only');
});

test('selecting a model saves the preference and confirms', async () => {
  const harness = makeClient();
  const container = await mountPanel(harness);

  const combobox = container.querySelector<HTMLButtonElement>('[role="combobox"]')!;
  await act(async () => combobox.click());
  const option = container.querySelector<HTMLButtonElement>('[role="option"][data-value="first-model"]')!;
  await act(async () => option.click());

  assert.deepEqual(harness.updates, [{ default_model: 'first-model' }]);
  const updated = container.querySelector<HTMLButtonElement>('[role="combobox"]')!;
  assert.equal(updated.getAttribute('data-value'), 'first-model');
  assert.ok(container.textContent?.includes('默认模型已保存'), 'the localized success notice renders');
});

test('the clear entry clears the preference with an empty-string patch', async () => {
  const harness = makeClient();
  const container = await mountPanel(harness);

  const combobox = container.querySelector<HTMLButtonElement>('[role="combobox"]')!;
  await act(async () => combobox.click());
  // The clearable entry is the first option in the popup (no data-value).
  const clear = container.querySelector<HTMLButtonElement>('[role="option"]')!;
  assert.equal(clear.getAttribute('data-value'), null, 'the clear option carries no data-value');
  await act(async () => clear.click());

  assert.deepEqual(harness.updates, [{ default_model: '' }]);
  assert.equal(container.querySelector<HTMLButtonElement>('[role="combobox"]')?.getAttribute('data-value'), '');
});

test('a failed save surfaces the localized error and keeps the prior value', async () => {
  const harness = makeClient({ failUpdate: true });
  const container = await mountPanel(harness);

  const combobox = container.querySelector<HTMLButtonElement>('[role="combobox"]')!;
  await act(async () => combobox.click());
  const option = container.querySelector<HTMLButtonElement>('[role="option"][data-value="first-model"]')!;
  await act(async () => option.click());

  assert.deepEqual(harness.updates, [], 'the failed update recorded nothing');
  assert.ok(container.textContent?.includes('保存默认模型失败'), 'the localized failure renders');
  assert.ok(container.textContent?.includes('preferences offline'), 'the backend message is passed through');
  assert.equal(container.querySelector<HTMLButtonElement>('[role="combobox"]')?.getAttribute('data-value'), 'glm-5.3', 'the echo stays on the previous value');
});
