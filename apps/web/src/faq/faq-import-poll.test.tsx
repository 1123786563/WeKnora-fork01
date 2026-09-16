// FAQ import polling (A2): after upsert returns a task_id the page polls
// faq.importProgress until completed, refreshes the list, then collapses the
// strip (Vue FAQEntryManager.vue:2091-2165 semantics).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => /\.(css|png|jpe?g|svg|gif|webp)$/.test(specifier)
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases/kb-1/faq' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { FAQPage } = await import('./FAQPage.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

function fakeClient(importProgressResults: Array<Record<string, unknown>>) {
  let progressCalls = 0;
  return {
    client: {
      knowledgeBases: {
        settings: { get: async () => ({ id: 'kb-1', user_id: 'u1', name: 'KB One', type: 'KnowledgeQA' }) },
        list: async () => [],
      },
      knowledge: {
        documents: { tags: async () => [] },
        faq: {
          list: async () => ({ data: [], total: 0, page: 1, page_size: 20 }),
          upsert: async () => ({ task_id: 'task-1' }),
          importProgress: async (taskId: string) => {
            const next = importProgressResults[Math.min(progressCalls, importProgressResults.length - 1)];
            progressCalls += 1;
            return { task_id: taskId, kb_id: 'kb-1', status: 'processing', progress: 40, total: 5, processed: 2, ...next };
          },
        },
      },
    auth: { me: async () => ({ user: { id: 'u1', roles: [], memberships: [] }, membership: { role: 'owner' }, can_access_all_tenants: false, knowledge_base: { user_id: 'u1' } }) },
    },
    progressCalls: () => progressCalls,
  };
}

async function mountPage(client: Record<string, unknown>) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(FAQPage, { client: client as never, knowledgeBaseId: 'kb-1' }));
  });
  await settle(20);
  return container;
}

test('faq import polls the task and collapses the strip on completion', async () => {
  const calls: string[] = [];
  const fake = fakeClient([
    { status: 'processing', progress: 40, processed: 2 },
    { status: 'completed', progress: 100, processed: 5 },
  ]);
  await mountPage(fake.client);

  // Open the import dialog and pick a JSON file.
  const dropdownItem = [...document.querySelectorAll('button, a')].find((n) => (n.textContent || '').includes('导入 FAQ')) as HTMLButtonElement | undefined;
  assert.ok(dropdownItem, 'expected the import dropdown item');
  await act(async () => { dropdownItem.click(); await settle(5); });
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  assert.ok(input, 'expected the import file input');
  const file = new dom.window.File([JSON.stringify([{ standard_question: 'Q?', answers: ['A'] }])], 'faq.json', { type: 'application/json' });
  Object.defineProperty(input, 'files', { value: [file] });
  await act(async () => { input.dispatchEvent(new window.Event('change', { bubbles: true })); await settle(5); });
  // The dialog confirm is the last button in the Vue-shaped import footer;
  // the header menu item only opens the dialog.
  const importButton = document.querySelector('.faq-import-footer button:last-child') as HTMLButtonElement | null;
  assert.ok(importButton, 'expected the import confirm button');
  await act(async () => { importButton.click(); await settle(30); });

  // The Vue poller starts on its 1.5s interval; the initial queued strip is
  // visible before the first server progress response arrives.
  await act(async () => { await settle(1700); });
  // Poll #1: processing (strip visible with counts).
  assert.ok(document.querySelector('.faq-import-strip--running'), 'expected the running strip');
  assert.match(document.querySelector('.faq-import-strip__count')?.textContent || '', /2\/5/);
  // Poll #2: completed strip appears, then the poll loop stops.
  await act(async () => { await settle(1700); });
  assert.ok(document.querySelector('.faq-import-strip--success'), 'expected the success strip after completion');
  assert.ok(fake.progressCalls() >= 2, 'polled at least twice');
  // Collapse after the 3s success window.
  await act(async () => { await settle(3200); });
  assert.equal(document.querySelector('.faq-import-strip'), null, 'strip collapses after success');
});

test('faq import polling failures remain visible as an error', async () => {
  const fake = fakeClient([]);
  fake.client.knowledge.faq.importProgress = async () => { throw new Error('导入进度服务不可用'); };
  await mountPage(fake.client);

  const dropdownItem = [...document.querySelectorAll('button, a')].find((n) => (n.textContent || '').includes('导入 FAQ')) as HTMLButtonElement | undefined;
  assert.ok(dropdownItem, 'expected the import dropdown item');
  await act(async () => { dropdownItem.click(); await settle(5); });
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  const file = new dom.window.File([JSON.stringify([{ standard_question: 'Q?', answers: ['A'] }])], 'faq.json', { type: 'application/json' });
  Object.defineProperty(input, 'files', { value: [file] });
  await act(async () => { input.dispatchEvent(new window.Event('change', { bubbles: true })); await settle(5); });
  const importButton = document.querySelector('.faq-import-footer button:last-child') as HTMLButtonElement | null;
  assert.ok(importButton, 'expected the import confirm button');
  await act(async () => { importButton.click(); await settle(30); });
  await act(async () => { await settle(1700); });

  assert.match(document.querySelector('main')?.textContent || '', /导入进度服务不可用/);
});
