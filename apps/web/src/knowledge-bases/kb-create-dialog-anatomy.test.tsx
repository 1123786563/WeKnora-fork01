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

function makeClient(): WeKnoraClient {
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
      create: async () => ({ id: 'kb-new' }),
      update: async () => ({}),
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
      },
    },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => [] } } },
  } as unknown as WeKnoraClient;
}

async function mountPage(): Promise<void> {
  const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' });
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeBasesPage client={makeClient()} scopeController={scopeController} />);
  });
  await act(async () => {});
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('[data-guide="kb-list-create"]')?.click();
  });
  await act(async () => {});
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
