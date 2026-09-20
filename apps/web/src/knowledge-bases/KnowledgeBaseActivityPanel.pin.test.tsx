// R485 H3 pin: the activity panel treats a zero next_cursor as exhaustion
// (Vue KnowledgeBaseActivitySettings.vue gates on !!next_cursor). Loading the
// final empty page must swap the load-more affordance for the end hint
// 「没有更早的记录了 / No earlier activity」 and never re-render Load more.
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
const { KnowledgeBaseActivityPanel } = await import('./KnowledgeBaseActivityPanel.tsx');

const firstPage = { data: [{ id: 2, action: 'kb.updated', outcome: 'success', created_at: '2026-09-19T10:00:00Z' }], next_cursor: 1 };
const finalPage = { data: [], next_cursor: 0 };

function clientWith(pages: unknown[]): WeKnoraClient {
  let call = 0;
  return {
    knowledgeBases: {
      settings: {
        activity: async () => pages[Math.min(call++, pages.length - 1)],
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

async function renderPanel(client: WeKnoraClient): Promise<void> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeBaseActivityPanel, { client, knowledgeBaseId: 'kb-1' }));
  });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

test('an exhausted cursor (next_cursor 0 with an empty page) renders the end hint and drops Load more', async () => {
  await renderPanel(clientWith([firstPage, finalPage]));
  const loadMore = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').includes('Load more'));
  assert.ok(loadMore, 'the first page with a live cursor offers Load more');
  assert.equal(document.body.textContent?.includes('No earlier activity'), false, 'no end hint while a cursor remains');
  await act(async () => { loadMore!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const text = document.body.textContent ?? '';
  assert.ok(text.includes('No earlier activity'), 'the end hint renders after the final empty page');
  assert.equal([...document.body.querySelectorAll('button')].some((candidate) => (candidate.textContent ?? '').includes('Load more')), false, 'the Load more affordance is gone once the cursor is exhausted');
});

// R489 M1 regression-sweep finding: the actor column must follow Vue
// KnowledgeBaseActivitySettings.actorLabel (L506-512) — the current user
// renders as their username/email, other actors render the actor_user_id
// 8-char prefix, and a missing id means the system actor. The full id never
// renders.
test('actor column follows the Vue actorLabel semantics (me → username, others → 8-char prefix)', async () => {
  const page = {
    data: [
      { id: 1, action: 'kb.updated', outcome: 'success', created_at: '2026-09-19T10:00:00Z', actor_user_id: 'u-me-1234' },
      { id: 2, action: 'kb.updated', outcome: 'success', created_at: '2026-09-19T10:00:01Z', actor_user_id: '3cd9521f-0e49-4c65-8cf5-f9b80b950e39' },
      { id: 3, action: 'kb.updated', outcome: 'success', created_at: '2026-09-19T10:00:02Z' },
    ],
    next_cursor: 0,
  };
  const client = {
    knowledgeBases: { settings: { activity: async () => page } },
    auth: { me: async () => ({ user: { id: 'u-me-1234', username: 'parity-test', email: 'parity-test@local.dev' } }) },
  } as unknown as WeKnoraClient;
  await renderPanel(client);
  const text = document.body.textContent ?? '';
  assert.ok(text.includes('parity-test'), 'the current user renders as their username');
  assert.ok(text.includes('3cd9521f'), 'another actor renders the 8-char id prefix');
  assert.equal(text.includes('3cd9521f-0e49'), false, 'the full actor id never renders');
});
