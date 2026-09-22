// R438 A2: knowledge-base settings surface IA phase 1 — grouped navigation
// mirroring Vue KnowledgeBaseEditorModal.vue navGroups (frontend/src/views/
// knowledge/KnowledgeBaseEditorModal.vue lines 637-669) and the modal chrome
// (.settings-modal 1000x750, .settings-sidebar 208px).
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
  // tdesign-react Select/Popup 运行时（parserSettings 平移为 tdesign Select）。
  Element: dom.window.Element,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Node: dom.window.Node,
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
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
const { getKnowledgeSettingsNavGroups, KnowledgeSettingsPage } = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

const documentKnowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  type: 'document',
  data_source_count: 3,
};

const faqKnowledgeBase: KnowledgeSettingsInput = { id: 'kb-2', name: 'FAQ base', type: 'faq' };

test('nav groups mirror the Vue editor 5-group 13-item contract for a document knowledge base', () => {
  const groups = getKnowledgeSettingsNavGroups(documentKnowledgeBase, { canViewActivity: true });
  assert.deepEqual(groups.map((group) => group.key), ['basic', 'processing', 'data', 'integration', 'management']);
  assert.deepEqual(groups.map((group) => group.labelKey), [
    'knowledgeEditor.navGroups.basic',
    'knowledgeEditor.navGroups.processing',
    'knowledgeEditor.navGroups.data',
    'knowledgeEditor.navGroups.integration',
    'knowledgeEditor.navGroups.management',
  ]);
  // pickItems order from KnowledgeBaseEditorModal.vue navGroups computed.
  assert.deepEqual(groups.map((group) => group.items.map((item) => item.key)), [
    ['basic', 'models', 'vectorStore'],
    ['parser', 'chunking', 'multimodal', 'asr', 'graph', 'advanced'],
    ['storage', 'datasource'],
    ['share'],
    ['activity'],
  ]);
  assert.equal(groups.reduce((total, group) => total + group.items.length, 0), 13);
});

test('nav item label keys reuse the Vue i18n contract (parser falls back to settings.parserEngine)', () => {
  const groups = getKnowledgeSettingsNavGroups(documentKnowledgeBase, { canViewActivity: true });
  const labelKeys = groups.flatMap((group) => group.items.map((item) => item.labelKey));
  assert.equal(labelKeys[0], 'knowledgeEditor.sidebar.basic');
  assert.equal(labelKeys.includes('settings.parserEngine'), true, 'parser label must reuse settings.parserEngine like Vue navItems');
  for (const labelKey of labelKeys.filter((key) => key !== 'settings.parserEngine')) {
    assert.match(labelKey, /^knowledgeEditor\.sidebar\./);
  }
  assert.equal(labelKeys.filter((key) => key === 'settings.parserEngine').length, 1);
});

test('FAQ knowledge bases collapse to the Vue basic group plus integration and management', () => {
  const groups = getKnowledgeSettingsNavGroups(faqKnowledgeBase, { canViewActivity: true });
  assert.deepEqual(groups.map((group) => group.key), ['basic', 'integration', 'management']);
  assert.deepEqual(groups[0]!.items.map((item) => item.key), ['basic', 'models', 'vectorStore', 'faq']);
});

test('empty groups drop out and activity gating removes the management group', () => {
  const groups = getKnowledgeSettingsNavGroups(documentKnowledgeBase, { canViewActivity: false });
  assert.deepEqual(groups.map((group) => group.key), ['basic', 'processing', 'data', 'integration']);
  assert.equal(groups.some((group) => group.items.some((item) => item.key === 'activity')), false);
});

test('datasource nav item carries the data-source count badge like the Vue editor', () => {
  const groups = getKnowledgeSettingsNavGroups(documentKnowledgeBase, { canViewActivity: true });
  const datasource = groups.flatMap((group) => group.items).find((item) => item.key === 'datasource');
  assert.equal(datasource?.badge, 3);
  const withoutSources = getKnowledgeSettingsNavGroups({ ...documentKnowledgeBase, data_source_count: 0 }, { canViewActivity: true });
  const plainDatasource = withoutSources.flatMap((group) => group.items).find((item) => item.key === 'datasource');
  assert.equal(plainDatasource?.badge, undefined);
});

// ---- Rendered surface ----

interface ClientCalls { dataSources: string[]; shareList: number; activityCalls: Array<{ knowledgeBaseId: string }> }

function clientFor(calls: ClientCalls): WeKnoraClient {
  return {
    request: async () => ({ data: [] }),
    dataSources: {
      list: async (kbId: string) => {
        calls.dataSources.push(kbId);
        return [{ id: 'ds-1', name: 'Feishu docs', type: 'feishu', status: 'completed' }];
      },
      types: async () => [],
    },
    knowledgeBases: {
      documents: {
        // Vue isIndexingLocked probe (loadKBData) — empty fixture keeps the
        // indexing checks unlocked.
        list: async () => ({ data: [], total: 0 }),
      },
      settings: {
        parserEngines: async () => ({ data: [{ Name: 'mineru', Description: 'MinerU self-hosted', Available: true, FileTypes: ['pdf'] }] }),
        storageBackends: async () => ({ data: [{ id: 'st-1', name: 'Main storage', provider: 's3', status: 'ready' }] }),
        vectorStores: async () => ({ data: [{ id: 'vs-1', name: 'Vectors', engine_type: 'pgvector', source: 'tenant', readonly: false }] }),
        activity: async (kbId: string, _query?: Record<string, unknown>) => {
          calls.activityCalls.push({ knowledgeBaseId: kbId });
          return { data: [{ id: 'a-1', action: 'kb.updated', outcome: 'success', created_at: '2026-09-15T00:00:00Z' }] };
        },
      },
    },
    identity: {
      organizations: {
        list: async () => {
          calls.shareList += 1;
          return { items: [], total: 0 };
        },
        knowledgeBaseShares: { list: async () => ({ items: [], total: 0 }) },
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient, knowledgeBase: KnowledgeSettingsInput = documentKnowledgeBase): Promise<void> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase, role: 'admin' }));
  });
  await act(async () => { await Promise.resolve(); });
}

function navButtons(): Array<HTMLButtonElement> {
  return [...document.body.querySelectorAll('button[data-section]')] as Array<HTMLButtonElement>;
}

function groupTitles(): string[] {
  return [...document.body.querySelectorAll('.wkbs-nav-group-title')].map((node) => node.textContent ?? '');
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

test('rendered sidebar shows the five Vue group titles in order with localized labels', async () => {
  await renderPage(clientFor({ dataSources: [], shareList: 0, activityCalls: [] }));
  assert.deepEqual(groupTitles(), ['Basics', 'Indexing & Parsing', 'Storage & Data', 'Publishing', 'Management & Audit']);
  assert.equal(navButtons().length, 13, 'all 13 Vue nav items must render (unported ones as placeholders)');
});

test('implemented sections are re-homed into their Vue groups: parser/storage under processing+data, share and activity last', async () => {
  await renderPage(clientFor({ dataSources: [], shareList: 0, activityCalls: [] }));
  const keys = navButtons().map((button) => button.getAttribute('data-section'));
  // Vue grouped order (KnowledgeBaseEditorModal.vue navGroups pickItems).
  assert.deepEqual(keys, [
    'basic', 'models', 'vectorStore',
    'parser', 'chunking', 'multimodal', 'asr', 'graph', 'advanced',
    'storage', 'datasource',
    'share',
    'activity',
  ]);
});

test('clicking a nav item activates it (Vue .nav-item.active) and swaps the content area', async () => {
  await renderPage(clientFor({ dataSources: [], shareList: 0, activityCalls: [] }));
  const storageButton = navButtons().find((button) => button.getAttribute('data-section') === 'storage');
  assert.ok(storageButton);
  await act(async () => { storageButton!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  assert.equal(storageButton!.className.includes('is-active'), true, 'active nav item must carry the is-active class');
  assert.equal(storageButton!.getAttribute('aria-current'), 'page');
  const activeCount = navButtons().filter((button) => button.className.includes('is-active')).length;
  assert.equal(activeCount, 1, 'exactly one nav item is active');
  assert.match(document.body.textContent ?? '', /Main storage/, 'storage section content renders after activation');

  const parserButton = navButtons().find((button) => button.getAttribute('data-section') === 'parser');
  await act(async () => { parserButton!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  assert.equal(storageButton!.className.includes('is-active'), false, 'previous item loses the active state');
  // tdesign Select 闭合态：选中项 label 走 trigger input 的 value，不出现在
  // textContent（原生 <option> 文本已随 <select> 移除）。
  const parserTrigger = document.body.querySelector('[data-parser-group="pdf"] .t-select__wrap');
  const parserValue = (parserTrigger?.querySelector('input.t-input__inner') as HTMLInputElement | null)?.value ?? '';
  assert.match(parserValue, /MinerU/);
});

test('R441 ports the basic section: the name editor renders instead of the placeholder', async () => {
  await renderPage(clientFor({ dataSources: [], shareList: 0, activityCalls: [] }));
  // R439 ported models/chunking/advanced; R440 ported multimodal/asr/faq;
  // R441 ports the Vue basic section (name/description/type/indexing), so no
  // nav item renders the shared not-yet-ported notice anymore.
  const basicButton = navButtons().find((button) => button.getAttribute('data-section') === 'basic');
  assert.ok(basicButton, 'basic nav item must exist (Vue renders it)');
  await act(async () => { basicButton!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  const text = document.body.textContent ?? '';
  assert.equal(text.includes('not been ported'), false, 'the basic section is fully ported');
  const nameInput = [...document.body.querySelectorAll('input')].find((candidate) => candidate.getAttribute('aria-label') === 'Knowledge Base Name') as HTMLInputElement | undefined;
  assert.ok(nameInput, 'the Vue name editor renders for the basic section');
  assert.equal(nameInput!.value, 'Product docs');
});

test('surface chrome matches the Vue modal: 1000x750 modal frame and 208px sidebar', async () => {
  await renderPage(clientFor({ dataSources: [], shareList: 0, activityCalls: [] }));
  const modal = document.body.querySelector('.wkbs-modal') as HTMLElement | null;
  assert.ok(modal, 'expected the .wkbs-modal Vue modal frame');
  assert.equal(modal.style.maxWidth, '1000px', 'Vue .settings-modal max-width');
  assert.equal(modal.style.maxHeight, '750px', 'Vue .settings-modal max-height');
  assert.equal(modal.style.width, '90vw');
  assert.equal(modal.style.height, '85vh');
  assert.equal(modal.style.borderRadius, '12px');
  const sidebar = document.body.querySelector('.wkbs-sidebar') as HTMLElement | null;
  assert.ok(sidebar, 'expected the Vue .settings-sidebar column');
  assert.equal(sidebar.style.width, '208px', 'Vue .settings-sidebar width');
});
