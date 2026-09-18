// FAQ route document-type gate + loading reset (R464 A2):
//   Vue KnowledgeBase.vue:88 `isFAQ = (kbInfo?.type || '') === 'faq'` — the FAQ
//   manager only mounts on the v-else branch of `v-if="!isFAQ"`, so a document
//   KB can never reach the FAQ view (Vue has no /faq route at all; the KB
//   detail always renders the documents view). React's standalone
//   /knowledgeBase/:id/faq route must mirror that gate: resolve the KB type
//   before any FAQ request, redirect non-FAQ KBs to the KB detail (documents)
//   route, and settle loading on failure so an empty list with hasMore stuck
//   true cannot re-fire the fill-short-page auto-append (the 400 retry storm).
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
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/knowledgeBase/kb-1/faq' });
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

// clientNavigate lands on window.history.{pushState,replaceState} — record
// every navigation so the redirect target and mode are assertable.
const navigations: Array<{ path: string; mode: 'push' | 'replace' }> = [];
for (const mode of ['pushState', 'replaceState'] as const) {
  const original = dom.window.history[mode].bind(dom.window.history);
  Object.defineProperty(dom.window.history, mode, {
    configurable: true,
    value: (...args: Parameters<History['pushState']>) => {
      navigations.push({ path: String(args[2]), mode: mode === 'pushState' ? 'push' : 'replace' });
      return original(...args);
    },
  });
}

const { createRoot } = await import('react-dom/client');
const { FAQPage } = await import('./FAQPage.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  navigations.length = 0;
  document.body.replaceChildren();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

const faqEntry = {
  id: 1,
  standard_question: '什么是 WeKnora？',
  similar_questions: [],
  negative_questions: [],
  answers: ['智能知识库管理系统'],
  is_enabled: true,
  is_recommended: false,
};

interface FakeOptions { kbType?: string; listError?: Error }

function fakeClient({ kbType = 'faq', listError }: FakeOptions = {}) {
  let listCalls = 0;
  let rawRequests = 0;
  return {
    client: {
      request: async () => { rawRequests += 1; return { success: true, data: null }; },
      knowledgeBases: {
        settings: { get: async () => ({ id: 'kb-1', user_id: 'u1', name: 'KB One', type: kbType }) },
        list: async () => [],
      },
      knowledge: {
        documents: { tags: async () => [] },
        faq: {
          // The pre-fix fill-short-page loop re-fires as an unbounded
          // microtask chain (render → effect → loadMore → …) that starves the
          // event loop, so a plain failing mock hangs the run. Stall the mock
          // after a few failures to make the storm countable; the fixed page
          // never reaches call 2.
          list: async () => {
            listCalls += 1;
            if (listCalls > 6) await new Promise(() => {});
            if (listError) throw listError;
            return { data: [faqEntry], total: 1, page: 1, page_size: 20 };
          },
          upsert: async () => ({ task_id: 'task-1' }),
          importProgress: async (taskId: string) => ({ task_id: taskId, kb_id: 'kb-1', status: 'completed', progress: 100, total: 1, processed: 1 }),
        },
      },
      auth: { me: async () => ({ user: { id: 'u1', roles: [], memberships: [] }, membership: { role: 'owner' }, can_access_all_tenants: false, knowledge_base: { user_id: 'u1' } }) },
    },
    listCalls: () => listCalls,
    rawRequests: () => rawRequests,
  };
}

async function mountPage(client: Record<string, unknown>) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(FAQPage, { client: client as never, knowledgeBaseId: 'kb-1' }));
  });
  await settle(50);
  return container;
}

test('document KB on the faq route redirects to the KB detail view and fires zero FAQ requests', async () => {
  const fake = fakeClient({ kbType: 'document' });
  const container = await mountPage(fake.client);

  assert.equal(fake.listCalls(), 0, 'faq.list must never be called for a document KB');
  assert.equal(fake.rawRequests(), 0, 'no raw /api/v1/faq/* request may fire for a document KB');
  const redirect = navigations.find((entry) => entry.path === '/knowledgeBase/kb-1');
  assert.ok(redirect, 'expected a redirect to the KB detail (documents) route /knowledgeBase/kb-1');
  assert.equal(redirect.mode, 'replace', 'the gate redirect replaces the unreachable /faq URL');
  assert.equal((container.textContent || '').trim(), '', 'no FAQ view markup renders for a document KB');
});

test('FAQ KB renders the manager and loads page 1 exactly once', async () => {
  const fake = fakeClient({ kbType: 'faq' });
  const container = await mountPage(fake.client);

  assert.equal(fake.listCalls(), 1, 'page 1 loads once');
  assert.ok(container.textContent?.includes('问答'), 'renders the Vue 问答 breadcrumb');
  assert.ok(container.querySelector('.faq-card'), 'renders the loaded FAQ entry card');
});

test('a failing list load settles loading and never auto-retries (no 400 storm)', async () => {
  const fake = fakeClient({ kbType: 'faq', listError: new Error('该知识库类型不支持 FAQ 接口') });
  const container = await mountPage(fake.client);
  // Long settle: the pre-fix defect re-fired the fill-short-page loadMore on
  // every render while hasMore stayed true, chaining requests indefinitely.
  await settle(300);

  assert.equal(fake.listCalls(), 1, 'exactly one request — the empty-list auto-append must stop after a failure');
  assert.equal(container.querySelector('.faq-card-skeleton'), null, 'loading skeleton must settle');
  assert.equal(container.querySelector('.faq-load-more'), null, 'the 加载中 load-more affordance must not linger');
  assert.match(container.textContent || '', /该知识库类型不支持 FAQ 接口/, 'the failure surfaces as an error message');
});
