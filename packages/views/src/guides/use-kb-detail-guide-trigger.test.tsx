// Component test for the detail-entry trigger of the kbDetail welcome tour.
// Vue baseline (read-only): frontend/src/views/knowledge/KnowledgeBase.vue
// mounts `<ContextualGuide tour="kbDetail" :when="showKbDetailContextualGuide" />`
// (line 2741) with the arm condition at lines 339-345, so the one-shot
// "knowledge base is empty" tour fires on detail-page entry whenever the
// condition turns true. The React hook fires the same openContextualGuide
// trigger; dismissal/welcome-tour gating stays inside the shell host
// (apps/web/src/platform/contextual-guide.test.tsx covers that side).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import * as React from 'react';
import test, { afterEach } from 'node:test';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import { OPEN_CONTEXTUAL_GUIDE_EVENT } from './contextual-guides.ts';
import { useKbDetailGuideTrigger, type KbDetailGuideTriggerInput } from './use-kb-detail-guide-trigger.ts';

// react-dom lives in the consuming app's node_modules (packages/views only
// declares the react peer dep), so pin both entrypoints there.
type ResolveFn = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveReactDom: ResolveFn = (specifier, context, nextResolve) => {
  if (specifier === 'react-dom/client') return { shortCircuit: true, url: new URL('../../../../apps/web/node_modules/react-dom/client.js', import.meta.url).href };
  if (specifier === 'react-dom') return { shortCircuit: true, url: new URL('../../../../apps/web/node_modules/react-dom/index.js', import.meta.url).href };
  return nextResolve(specifier, context);
};
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveFn }) => void };
hooks.registerHooks?.({ resolve: resolveReactDom });
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/knowledgeBase/kb-1' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// openContextualGuide defaults to the global sessionStorage for the pending
// hand-off; jsdom provides it on window, tests read it through the same object.
const sessionStorage = dom.window.sessionStorage;

const { createRoot } = await import('react-dom/client');

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  sessionStorage.clear();
});

function armedInput(overrides: Partial<KbDetailGuideTriggerInput> = {}): KbDetailGuideTriggerInput {
  return { knowledgeBaseId: 'kb-1', kbType: 'document', canEdit: true, documentsLoading: false, documentCount: 0, ...overrides };
}

/** Records the trigger events + pending intent a render pass produced. */
async function renderWithInput(input: KbDetailGuideTriggerInput): Promise<{ events: string[]; pending: string | null }> {
  const events: string[] = [];
  const onEvent = (event: Event) => events.push(`${event.type}:${String((event as CustomEvent<{ tour: string }>).detail.tour)}`);
  dom.window.addEventListener(OPEN_CONTEXTUAL_GUIDE_EVENT, onEvent);
  const container = document.createElement('div');
  const root = createRoot(container);
  try {
    function HookHost(props: { input: KbDetailGuideTriggerInput }) {
      useKbDetailGuideTrigger(props.input);
      return null;
    }
    await act(async () => {
      root.render(React.createElement(HookHost, { input }));
    });
    await act(async () => { await Promise.resolve(); });
  } finally {
    dom.window.removeEventListener(OPEN_CONTEXTUAL_GUIDE_EVENT, onEvent);
    await act(async () => root.unmount());
    container.remove();
  }
  return { events, pending: sessionStorage.getItem('weknora:contextual-guide-pending:v1') };
}

test('(kbDetail entry trigger) arms on detail entry for an editable, non-FAQ, empty KB', async () => {
  const { events, pending } = await renderWithInput(armedInput());
  assert.deepEqual(events, [`${OPEN_CONTEXTUAL_GUIDE_EVENT}:kbDetail`]);
  const parsed = JSON.parse(pending ?? 'null') as { tour?: string } | null;
  assert.equal(parsed?.tour, 'kbDetail', 'pending intent is recorded for the shell host hand-off');
});

test('(kbDetail entry trigger) stays silent while loading, for viewers, FAQ libs and non-empty KBs', async () => {
  for (const [label, overrides] of [
    ['documents still loading', { documentsLoading: true }],
    ['viewer without edit rights', { canEdit: false }],
    ['FAQ library', { kbType: 'faq' }],
    ['non-empty KB', { documentCount: 2 }],
  ] as const) {
    const { events, pending } = await renderWithInput(armedInput({ ...overrides }));
    assert.deepEqual(events, [], `no trigger for ${label}`);
    assert.equal(pending, null, `no pending intent for ${label}`);
  }
});

test('(kbDetail entry trigger) refires when the arm condition turns true again or the KB id changes', async () => {
  const seen: string[] = [];
  const onEvent = (event: Event) => seen.push(String((event as CustomEvent<{ tour: string }>).detail.tour));
  dom.window.addEventListener(OPEN_CONTEXTUAL_GUIDE_EVENT, onEvent);
  function HookHost(props: { input: KbDetailGuideTriggerInput }) {
    useKbDetailGuideTrigger(props.input);
    return null;
  }
  const container = document.createElement('div');
  mountedRoot = createRoot(container);
  const rerender = async (input: KbDetailGuideTriggerInput) => {
    await act(async () => {
      mountedRoot?.render(React.createElement(HookHost, { input, key: input.knowledgeBaseId }));
    });
    await act(async () => { await Promise.resolve(); });
  };

  try {
    await rerender(armedInput());
    assert.deepEqual(seen, ['kbDetail'], 'initial entry arms');
    // A document upload lands: the condition drops, no re-fire on re-render.
    await rerender(armedInput({ documentCount: 1 }));
    assert.deepEqual(seen, ['kbDetail']);
    // Back to empty (delete) with the same KB id: condition true again re-fires,
    // matching the Vue watch on `when` re-scheduling after a reset.
    await rerender(armedInput());
    assert.deepEqual(seen, ['kbDetail', 'kbDetail']);
    // Same condition, different KB id (in-document KB switch): re-arms.
    await rerender(armedInput({ knowledgeBaseId: 'kb-2' }));
    assert.deepEqual(seen, ['kbDetail', 'kbDetail', 'kbDetail']);
  } finally {
    dom.window.removeEventListener(OPEN_CONTEXTUAL_GUIDE_EVENT, onEvent);
  }
});
