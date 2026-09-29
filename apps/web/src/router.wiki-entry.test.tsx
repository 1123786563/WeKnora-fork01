// Wiki entry gate regression tests (Vue parity): a knowledge base without the
// wiki capability visited at a wiki URL must fall back to the documents view
// (Vue KnowledgeBase.vue `!isWiki` branch) without touching history during the
// render pass. The production crash this pins down: WikiEntry used to call
// window.history.replaceState inside its render body, and both the navigation
// observer (platform/navigation.ts syncRouter) and @tanstack's browser history
// patch replaceState to notify the router synchronously — a render-phase
// notify setStates the router while WikiEntry is still rendering
// ("Cannot update a component while rendering a different component" →
// render-process crash loop on wiki-disabled libraries).
//
// The heavy lazy pages (KnowledgeDocumentsPage/WikiPage) are swapped for tiny
// stubs through the module resolve hook, so the test exercises the gate, not
// the documents surface.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };

// Lazy-import stand-ins: plain-JS data: modules whose createElement rides the
// test's globalThis.React (the classic-JSX global precedent).
function stubModuleUrl(exportName: string, marker: string): string {
  const source = `export function ${exportName}(props) { const R = globalThis.React; return R.createElement('div', { 'data-wiki-entry': ${JSON.stringify(marker)} }, ${JSON.stringify(marker)} + ':' + props.knowledgeBaseId); }`;
  return `data:text/javascript,${encodeURIComponent(source)}`;
}
const fromRouter = (context: unknown): string => String((context as { parentURL?: unknown }).parentURL ?? '');
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => {
  if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
  if (specifier === './documents/KnowledgeDocumentsPage.tsx' && fromRouter(context).endsWith('/router.tsx')) return { shortCircuit: true, url: stubModuleUrl('KnowledgeDocumentsPage', 'documents') };
  if (specifier === './wiki/WikiPage.tsx' && fromRouter(context).endsWith('/router.tsx')) return { shortCircuit: true, url: stubModuleUrl('WikiPage', 'wiki') };
  return nextResolve(specifier, context);
} });

import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/knowledgeBase/kb-1/wiki?knowledge_id=doc-9' });
(globalThis as typeof globalThis & { window: unknown; document: unknown }).window = dom.window;
(globalThis as typeof globalThis & { window: unknown; document: unknown }).document = dom.window.document;

import * as React from 'react';
import { createRoot } from 'react-dom/client';

(Object.assign as (target: unknown, patch: Record<string, unknown>) => unknown)(globalThis, { React });
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// Patched history: like navigation.ts / @tanstack/history, replaceState
// synchronously notifies the router after committing the URL — the notify is
// what must never fire while a component is rendering.
const replaceCalls: string[] = [];
let notifyRouter: (() => void) | undefined;
let notifyBudget = 3; // bounded so a reintroduced render-phase loop fails fast, not by timeout
const originalReplaceState = dom.window.history.replaceState.bind(dom.window.history);
dom.window.history.replaceState = ((_state: unknown, _title: string, url: string | URL | null | undefined) => {
  originalReplaceState(_state, _title, typeof url === 'string' ? url : undefined);
  replaceCalls.push(String(url));
  if (notifyBudget-- > 0) notifyRouter?.();
}) as History['replaceState'];

// Sibling stand-in for the router Transitioner: the patched replaceState
// setStates it, exactly like a router re-dispatch would.
function RouterProbe(): null {
  const [, bump] = React.useState(0);
  notifyRouter = () => bump((count) => count + 1);
  return null;
}

// Capture the React warning that named the production crash.
const consoleErrors: string[] = [];
const originalConsoleError = console.error.bind(console);
console.error = (...args: unknown[]) => {
  consoleErrors.push(args.map((arg) => String(arg)).join(' '));
  originalConsoleError(...args);
};

const { WikiEntry } = await import('./router.tsx');

interface GateCase {
  name: string;
  settings: () => Promise<unknown>;
  expectedView: string;
  expectedReplace?: string[];
}

async function mountGate(testCase: GateCase): Promise<HTMLElement> {
  const client = { knowledgeBases: { settings: { get: testCase.settings } } };
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  consoleErrors.length = 0;
  replaceCalls.length = 0;
  notifyBudget = 3;
  await React.act(async () => {
    root.render(
      React.createElement(React.Suspense, { fallback: 'loading' },
        React.createElement(RouterProbe),
        React.createElement(WikiEntry, { client: client as never, knowledgeBaseId: 'kb-1', initialDocumentId: 'doc-9', canContribute: false }),
      ),
    );
    await new Promise((resolve) => setTimeout(resolve, 20));
  });
  const view = container.querySelector<HTMLElement>(`[data-wiki-entry="${testCase.expectedView}"]`);
  assert.ok(view, `expected the ${testCase.expectedView} view to render, got ${container.innerHTML}`);
  return container;
}

function assertNoRenderPhaseNotify(): void {
  assert.ok(
    consoleErrors.every((message) => !message.includes('Cannot update a component while rendering a different component')),
    `replaceState must not notify the router during render, got: ${consoleErrors.join(' | ')}`,
  );
}

const gateCases: GateCase[] = [
  {
    name: 'a wiki-disabled library falls back to the documents view and cleans the URL after commit',
    settings: () => Promise.resolve({ id: 'kb-1', indexing_strategy: { wiki_enabled: false } }),
    expectedView: 'documents',
    expectedReplace: ['/knowledgeBase/kb-1'],
  },
  {
    name: 'a wiki-enabled library renders the wiki view without touching history',
    settings: () => Promise.resolve({ id: 'kb-1', indexing_strategy: { wiki_enabled: true } }),
    expectedView: 'wiki',
    expectedReplace: [],
  },
  {
    // Preserve the existing error surface: an unavailable capability lookup
    // keeps the wiki view (its own error handling) instead of the documents.
    name: 'a failed capability lookup keeps the wiki error surface',
    settings: () => Promise.reject(new Error('settings unavailable')),
    expectedView: 'wiki',
    expectedReplace: [],
  },
];

for (const testCase of gateCases) {
  test(`wiki entry gate: ${testCase.name}`, async () => {
    const container = await mountGate(testCase);
    assertNoRenderPhaseNotify();
    assert.deepEqual(replaceCalls, testCase.expectedReplace, 'only the post-commit address-bar cleanup may write history');
    container.remove();
  });
}
