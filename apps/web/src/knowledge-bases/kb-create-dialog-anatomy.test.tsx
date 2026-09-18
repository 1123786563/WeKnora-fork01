import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain';

// R430 kb-create dialog anatomy slice (Vue KnowledgeBaseEditorModal.vue authority):
//   (a) type selector keeps the joined TDesign outline radio-group frame
//       (buttons carry the 1px #e7e7e7 border + 3px end radius; checked tab
//       is the brand-green fill with the green shared divider; arrow keys move
//       the selection like t-radio-group)
//   (b) the Wiki indexing card carries the NEW badge (brand-light pill,
//       10px/600, 16px tall, 3px radius, 6px x-padding)
//   (c) the description textarea carries the TDesign "0/200" limit counter
//       (right-aligned, 12px/20px placeholder gray, native maxlength=200)
// R463 kb-editor submit-structure slice (same Vue authority):
//   (d) the editor dialog renders NO wrapping <form> — the Vue modal has
//       zero native form elements (footer buttons are plain @click handlers,
//       :448-455), so the wk-form grid is carried by a plain <div> and the
//       nested-form hydration error source is gone at the root
//   (e) the save button submits through onClick into the same save pipeline
//       (Vue :451 `<t-button @click="handleSubmit">`), not type="submit"
//   (f) Enter in the name input does NOT submit (Vue has no form → no
//       implicit-submission contract) and the name input carries no native
//       `required` attribute (Vue t-input :165-169 is maxlength-only; blank
//       names are blocked by the JS pipeline, `if (!name.trim()) return`)
// R466 loading-disable slice (same Vue authority :451-454):
//   (g) the footer save button is `:disabled="loading"` — while the editor's
//       data (models/storage/vector/parser options) is still loading the save
//       button is disabled with the SAME label (Vue only disables, the label
//       stays saveButtonLabel), and the cancel button (:448) carries no
//       loading disable so it stays clickable
//   (h) `saving` keeps its own semantics independent of `loading`: once the
//       editor data has settled, save is enabled; during the submit request
//       the button shows the saving disable+spinner (aria-busy) and the
//       double-submit guard keeps exactly one create call

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg?raw') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// openContextualGuide builds `new CustomEvent(...)` from the Node global and
// hands it to jsdom's window.dispatchEvent, which rejects foreign-realm
// events; route the Node global through the jsdom window for this harness.
(globalThis as { CustomEvent?: unknown }).CustomEvent = dom.window.CustomEvent;
// The live parity environment runs zh-CN (locale seeded via localStorage); the
// page resolves its locale from navigator.language, so pin it for assertions.
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });

const { createRoot } = await import('react-dom/client');
const { KnowledgeBasesPage } = await import('../App.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.history.replaceState(null, '', '/platform/knowledge-bases');
  dom.window.localStorage.clear();
});

function makeClient(calls: string[] = []): WeKnoraClient {
  return {
    auth: {
      me: async () => ({ user: { id: 'u-1', is_system_admin: false }, memberships: [{ tenant_id: 't-1', role: 'contributor' }] }),
    },
    knowledgeBases: {
      list: async () => [
        { id: 'kb-doc', name: 'Parity KB Demo', description: 'parity test data', type: 'document', knowledge_count: 2, creator_id: 'u-1', summary_model_id: 'm-1', embedding_model_id: 'm-2' },
      ],
      togglePin: async () => ({ is_pinned: true }),
      duplicate: async () => ({}),
      remove: async () => ({}),
      create: async () => { calls.push('create'); return { id: 'kb-new' }; },
      update: async () => { calls.push('update'); return {}; },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
      },
    },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => [] } } },
  } as unknown as WeKnoraClient;
}

async function mountPageWithClient(client: WeKnoraClient): Promise<void> {
  const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' });
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
  });
  await act(async () => {});
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('[data-guide="kb-list-create"]')?.click();
  });
  await act(async () => {});
}

async function mountPage(calls: string[] = []): Promise<void> {
  await mountPageWithClient(makeClient(calls));
}

function getTypeFrame(): HTMLElement | null {
  return document.body.querySelector<HTMLElement>('[data-guide="kb-create-type"] .kb-create-type-frame');
}

test('(a) the type selector keeps the joined TDesign outline frame with the brand-green checked tab', async () => {
  await mountPage();
  const frame = getTypeFrame();
  assert.ok(frame, 'expected the .kb-create-type-frame wrapper inside [data-guide="kb-create-type"]');
  assert.match(frame.className, /(^|\s)rounded-\[3px\]/, 'frame radius follows Vue --td-radius-default (3px)');
  assert.ok(!/(^|\s)border(\s|$)/.test(frame.className), 'the frame itself adds no wrapper border: the joined buttons carry the TDesign outline');

  const radios = Array.from(frame.querySelectorAll<HTMLButtonElement>('[role="radio"]'));
  assert.deepEqual(radios.map((radio) => radio.textContent), ['文档', '问答']);
  const [documentRadio, faqRadio] = radios;
  assert.equal(documentRadio.getAttribute('aria-checked'), 'true', 'document is the create default');
  assert.match(documentRadio.className, /bg-accent/, 'checked tab uses the brand fill (Vue theme.css green fill override)');
  assert.match(documentRadio.className, /border-\[var\(--color-brand\)\]/, 'checked tab border is brand so the shared divider reads green');
  assert.match(documentRadio.className, /(^|\s)rounded-l-\[3px\]/);
  assert.match(faqRadio.className, /(^|\s)rounded-r-\[3px\]/);
  assert.ok(!/(^|\s)border-l(\s|$)/.test(faqRadio.className), 'the tab after the checked one drops its left border so the divider stays a single line');
  assert.match(faqRadio.className, /border-line-neutral/, 'unchecked tabs keep the #e7e7e7 component-stroke border');
  assert.equal(faqRadio.tabIndex, -1, 'roving tabindex mirrors the radio-group contract');
  assert.equal(documentRadio.tabIndex, 0);
});

test('(a) arrow keys move the type selection like the TDesign radio group', async () => {
  await mountPage();
  const frame = getTypeFrame();
  assert.ok(frame);
  await act(async () => {
    frame.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
  });
  await act(async () => {});
  // Selecting FAQ flips the section content (问答配置) exactly like clicking the tab.
  const faqHeading = Array.from(document.body.querySelectorAll('h3')).find((el) => (el.textContent ?? '') === '问答');
  assert.ok(faqHeading, 'arrow key selection landed on the FAQ section');
});

test('(b) the Wiki indexing card carries the NEW badge and the RAG card does not', async () => {
  await mountPage();
  const cards = Array.from(document.body.querySelectorAll('fieldset[data-guide="kb-create-indexing"] label'));
  assert.equal(cards.length, 2, 'RAG + Wiki cards render');
  const wikiCard = cards.find((card) => (card.textContent ?? '').includes('Wiki 知识库'));
  const ragCard = cards.find((card) => (card.textContent ?? '').includes('RAG 检索'));
  assert.ok(wikiCard && ragCard, 'both indexing cards found');

  const badge = wikiCard.querySelector('.kb-editor-new-badge');
  assert.ok(badge, 'wiki card renders the NEW badge');
  assert.equal(badge.textContent, 'NEW');
  assert.match(badge.className, /bg-\[var\(--color-brand-light\)\]/, 'badge background is the brand-light pill (#e9f8ec)');
  assert.match(badge.className, /(^|\s)rounded-\[3px\]/, 'badge radius 3px');
  assert.match(badge.className, /(^|\s)h-4/, 'badge height 16px');
  assert.match(badge.className, /px-\[6px\]/, 'badge x-padding 6px');
  assert.match(badge.className, /text-\[10px\]/, 'badge font-size 10px');
  assert.match(badge.className, /font-semibold/, 'badge weight 600');
  assert.match(badge.className, /tracking-\[0\.4px\]/, 'badge letter-spacing 0.4px');
  assert.equal(ragCard.querySelector('.kb-editor-new-badge'), null, 'RAG card carries no NEW badge');
});

test('(c) the description textarea carries the live 0/200 limit counter', async () => {
  await mountPage();
  const textarea = document.body.querySelector<HTMLTextAreaElement>('textarea[placeholder="请输入知识库描述（可选）"]');
  assert.ok(textarea, 'description textarea rendered');
  assert.equal(textarea.getAttribute('maxlength'), '200', 'native maxlength=200 truncates input like Vue t-textarea');

  const counter = textarea.closest('label')?.querySelector('.kb-editor-desc-count');
  assert.ok(counter, 'limit counter rendered below the textarea');
  assert.equal(counter.textContent, '0/200');
  assert.match(counter.className, /justify-self-end/, 'counter right-aligned like .t-textarea__info_wrapper_align');
  assert.match(counter.className, /text-xs/, 'counter font-size 12px like .t-textarea__limit');
  assert.match(counter.className, /text-\[var\(--color-text-placeholder\)\]/, 'counter uses the placeholder gray');
  assert.equal(counter.getAttribute('aria-live'), 'polite', 'counter announces updates to assistive tech');

  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLTextAreaElement.prototype, 'value')?.set;
  await act(async () => {
    setValue?.call(textarea, '测试描述');
    textarea.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {});
  assert.equal(counter.textContent, '4/200', 'counter follows the input value');
});

test('(d) the editor dialog renders no wrapping form — Vue KnowledgeBaseEditorModal has zero <form> elements', async () => {
  await mountPage();
  assert.equal(document.body.querySelectorAll('form').length, 0, 'no native form wraps the editor sections (removes the form-in-form hydration error source)');
  const wrapper = document.body.querySelector('.wk-kb-editor-dialog .wk-form');
  assert.ok(wrapper, 'the wk-form grid wrapper still renders for section layout');
  assert.equal(wrapper.tagName, 'DIV', 'the wk-form grid is carried by a plain div like the Vue settings-body');
});

test('(d-edit) edit mode: visiting every section (share included) keeps the DOM form-free', async () => {
  await mountPage();
  // Card settings menu → 知识库设置 opens the editor in edit mode.
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('.kb-list-card-more')?.click();
  });
  await act(async () => {});
  const settingsItem = Array.from(document.body.querySelectorAll('[role="menuitem"]'))
    .find((el) => (el.textContent ?? '') === '设置');
  assert.ok(settingsItem, 'card settings menu item rendered');
  await act(async () => {
    (settingsItem as HTMLButtonElement).click();
  });
  await act(async () => {});
  const navButtons = Array.from(document.body.querySelectorAll<HTMLButtonElement>('[data-guide^="kb-editor-nav-"]'))
    // The datasource section mounts DataSourcesPage, whose client mock surface
    // (datasource types etc.) is out of scope for this anatomy slice.
    .filter((button) => button.dataset.guide !== 'kb-editor-nav-datasource');
  assert.ok(navButtons.length > 0, 'editor sidebar nav rendered');
  assert.ok(navButtons.some((button) => button.dataset.guide === 'kb-editor-nav-share'), 'the share section (the R462 nested-form site) is visitable');
  for (const button of navButtons) {
    await act(async () => {
      button.click();
    });
    await act(async () => {});
    assert.equal(document.body.querySelectorAll('form').length, 0, `section ${button.dataset.guide} renders no form — nested-form hydration errors are structurally impossible`);
  }
});

test('(e) the save button submits the create pipeline via onClick like Vue @click="handleSubmit"', async () => {
  const calls: string[] = [];
  await mountPage(calls);
  const button = document.body.querySelector<HTMLButtonElement>('[data-guide="kb-create-submit"]');
  assert.ok(button, 'save button rendered');
  assert.notEqual(button.type, 'submit', 'Vue footer buttons are plain @click handlers, not type="submit"');

  const nameInput = document.body.querySelector<HTMLInputElement>('input[data-guide="kb-create-name"]');
  assert.ok(nameInput, 'name input rendered on the default basic section');
  const setInputValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  await act(async () => {
    setInputValue?.call(nameInput, 'R463 提交结构');
    nameInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {});
  // The embedding field lives on the models section — navigate there like a
  // user would before filling it.
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('[data-guide="kb-editor-nav-models"]')?.click();
  });
  await act(async () => {});
  const embeddingInput = document.body.querySelector<HTMLInputElement>('input[data-guide="kb-create-embedding"]');
  assert.ok(embeddingInput, 'embedding input rendered after switching to the models section');
  const summaryInput = document.body.querySelector<HTMLInputElement>('input[data-guide="kb-create-llm"]');
  assert.ok(summaryInput, 'summary model input rendered on the models section');
  await act(async () => {
    setInputValue?.call(embeddingInput, 'm-embed');
    embeddingInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    setInputValue?.call(summaryInput, 'm-summary');
    summaryInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {});
  await act(async () => {
    button.click();
  });
  await act(async () => {});
  assert.deepEqual(calls, ['create'], 'the save pipeline reaches knowledgeBases.create exactly once');
});

test('(f) Enter in the name input does not submit and the input carries no native required attribute', async () => {
  const calls: string[] = [];
  await mountPage(calls);
  const nameInput = document.body.querySelector<HTMLInputElement>('input[data-guide="kb-create-name"]');
  assert.ok(nameInput);
  assert.equal(nameInput.required, false, 'Vue t-input (:165-169) is maxlength-only; blank names are blocked in the JS pipeline');
  assert.equal(nameInput.getAttribute('maxlength'), '50');

  const setInputValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  await act(async () => {
    setInputValue?.call(nameInput, 'R463 回车不提交');
    nameInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {
    nameInput.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  });
  await act(async () => {});
  assert.deepEqual(calls, [], 'no implicit Enter submission contract — Vue has no form to submit');
});

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => { resolve = res; });
  return { promise, resolve };
}

function getFooterCancelButton(saveButton: HTMLButtonElement): HTMLButtonElement | undefined {
  return Array.from(saveButton.parentElement?.querySelectorAll<HTMLButtonElement>('button') ?? [])
    .find((button) => (button.textContent ?? '') === '取消');
}

test('(g) the save button is disabled (label unchanged) while editor data loads; the cancel button stays enabled', async () => {
  // Vue KnowledgeBaseEditorModal.vue watch(visible) sets loading=true for every
  // open and clears it in loadKBData's finally — the footer mirrors that with
  // :disabled="loading". React's counterpart is editorOptions.loading, armed by
  // loadEditorOptions() on both openCreate and openEdit.
  const settingsGate = deferred<void>();
  const client = makeClient();
  (client.knowledgeBases as unknown as { settings: unknown }).settings = {
    parserEngines: () => settingsGate.promise.then(() => ({ data: [] })),
    storageBackends: () => settingsGate.promise.then(() => ({ data: [] })),
    vectorStores: () => settingsGate.promise.then(() => ({ data: [] })),
  };
  await mountPageWithClient(client);

  const saveButton = document.body.querySelector<HTMLButtonElement>('[data-guide="kb-create-submit"]');
  assert.ok(saveButton, 'save button rendered');
  assert.equal(saveButton.disabled, true, 'Vue :452 :disabled="loading" — editor data not ready blocks save');
  assert.equal(saveButton.getAttribute('aria-busy'), null, 'loading-disable is a plain disable, not the saving spinner');
  assert.equal(saveButton.textContent, '创建知识库', 'Vue keeps saveButtonLabel while loading (disable-only, no relabel)');

  const cancelButton = getFooterCancelButton(saveButton);
  assert.ok(cancelButton, 'footer cancel button rendered');
  assert.equal(cancelButton.disabled, false, 'Vue cancel (:448-450) has no loading binding — stays clickable while loading');

  await act(async () => { settingsGate.resolve(); });
  await act(async () => {});
  assert.equal(saveButton.disabled, false, 'save re-enables once the editor data settles (Vue loading=false)');
});

test('(h) saving keeps its own disable+aria-busy semantics once loading has settled', async () => {
  const calls: string[] = [];
  const createGate = deferred<void>();
  const client = makeClient(calls);
  (client.knowledgeBases as unknown as { create: unknown }).create = () => createGate.promise.then(() => { calls.push('create'); return { id: 'kb-new' }; });
  await mountPageWithClient(client);

  const saveButton = document.body.querySelector<HTMLButtonElement>('[data-guide="kb-create-submit"]');
  assert.ok(saveButton, 'save button rendered');
  assert.equal(saveButton.disabled, false, 'settings settled → loading no longer disables save');

  const setInputValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  const nameInput = document.body.querySelector<HTMLInputElement>('input[data-guide="kb-create-name"]');
  assert.ok(nameInput, 'name input rendered');
  await act(async () => {
    setInputValue?.call(nameInput, 'R466 保存态独立');
    nameInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('[data-guide="kb-editor-nav-models"]')?.click();
  });
  await act(async () => {});
  const embeddingInput = document.body.querySelector<HTMLInputElement>('input[data-guide="kb-create-embedding"]');
  const summaryInput = document.body.querySelector<HTMLInputElement>('input[data-guide="kb-create-llm"]');
  assert.ok(embeddingInput && summaryInput, 'model inputs rendered on the models section');
  await act(async () => {
    setInputValue?.call(embeddingInput, 'm-embed');
    embeddingInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    setInputValue?.call(summaryInput, 'm-summary');
    summaryInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {});

  await act(async () => {
    saveButton.click();
  });
  await act(async () => {});
  assert.equal(saveButton.disabled, true, 'saving disables the button (Vue :loading="saving")');
  assert.equal(saveButton.getAttribute('aria-busy'), 'true', 'saving carries the spinner affordance, distinct from the loading disable');
  await act(async () => {
    saveButton.click();
  });
  await act(async () => {});

  await act(async () => { createGate.resolve(); });
  await act(async () => {});
  assert.deepEqual(calls, ['create'], 'double-submit guard holds: exactly one create while saving');
  assert.ok(!saveButton.isConnected || !saveButton.disabled, 'a successful save closes the dialog (button gone or re-enabled)');
});
