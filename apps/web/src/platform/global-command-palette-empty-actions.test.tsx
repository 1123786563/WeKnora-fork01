// R465-A2 — empty-state action buttons + agentsEnabled deployment gating in
// the React GlobalCommandPalette. Vue contract (GlobalCommandPalette.vue):
//   • Lines 126-138: the no-results empty state carries TWO buttons —
//     "Ask the AI directly" (askAi: pushRecent + close + startChat(query) →
//     new chat session seeded with the current query) and "Adjust retrieval"
//     (adjustRetrieval: opens the layered RetrievalSettings drawer).
//   • Lines 153-157: the drawer is a 420px right panel titled
//     retrievalSettings.title hosting the RetrievalSettings form.
//   • Lines 205 + 262-270: `agentsEnabled: deploymentCapabilities
//     .isSupported('agents')` gates the agent search group, and the
//     open-agents / open-organizations quick actions are filtered by the
//     same deployment capabilities (organizations additionally requires an
//     admin role).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
  requestAnimationFrame: (cb: (time: number) => void) => setTimeout(() => cb(16), 0) as unknown as number,
  cancelAnimationFrame: (id: number) => clearTimeout(id),
});

const { createRoot } = await import('react-dom/client');
const { GlobalCommandPalette } = await import('./GlobalCommandPalette.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

/** Bare live-search client whose every endpoint returns zero hits. */
function emptyClient(calls: { agents: number }): object {
  return {
    knowledgeBases: {
      list: async () => [{ id: 'kb-1', name: 'Alpha KB' }],
      search: async () => [],
    },
    settings: {
      chatHistory: {
        search: async () => ({ items: [], total: 0 }),
      },
    },
    sessions: {
      list: async () => ({ data: [] }),
    },
    configuration: {
      agents: {
        list: async () => { calls.agents += 1; return []; },
      },
    },
  };
}

async function mountPalette(extra: Record<string, unknown> = {}): Promise<{
  spies: {
    onClose: unknown[];
    onNavigate: string[];
    onSearch: string[];
    onAskAi: string[];
  };
}> {
  const spies = {
    onClose: [] as unknown[],
    onNavigate: [] as string[],
    onSearch: [] as string[],
    onAskAi: [] as string[],
  };
  const props = {
    open: true,
    initialQuery: '',
    recentQueries: [],
    locale: 'en-US' as const,
    onClose: () => { spies.onClose.push(true); },
    onNavigate: (path: string) => { spies.onNavigate.push(path); },
    onSearch: (query: string) => { spies.onSearch.push(query); },
    onClearRecent: () => {},
    searchDebounceMs: 10,
    ...extra,
  };
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(GlobalCommandPalette, props));
  });
  await settle(5);
  return { spies };
}

const input = () => document.querySelector<HTMLInputElement>('.cmdk__input');

const typeQuery = (value: string) => act(async () => {
  const el = input();
  if (!el) throw new Error('palette input not mounted');
  const setter = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  setter?.call(el, value);
  el.dispatchEvent(new window.Event('input', { bubbles: true }));
});

const click = (el: Element) => act(async () => {
  el.dispatchEvent(new window.MouseEvent('click', { bubbles: true, cancelable: true }));
});

const buttonByText = (text: string): HTMLButtonElement | null => {
  for (const btn of Array.from(document.querySelectorAll('button'))) {
    if ((btn.textContent ?? '').trim() === text) return btn;
  }
  return null;
};

async function driveToEmptyState(extra: Record<string, unknown> = {}): Promise<ReturnType<typeof mountPalette>> {
  const mounted = await mountPalette(extra);
  await typeQuery('zzz-nothing-matches');
  await settle(30);
  return mounted;
}

// ─── Empty-state buttons ───

test('settled no-results state renders the askAi and adjustRetrieval buttons (Vue empty actions)', async () => {
  const calls = { agents: 0 };
  await driveToEmptyState({ searchClient: emptyClient(calls), retrievalSettings: <p data-testid="retrieval-body">retrieval form</p> });
  assert.ok(buttonByText('Ask the AI directly'), 'askAi button rendered');
  assert.ok(buttonByText('Adjust retrieval'), 'adjustRetrieval button rendered');
});

test('adjustRetrieval stays hidden when the shell provides no retrieval settings surface', async () => {
  const calls = { agents: 0 };
  await driveToEmptyState({ searchClient: emptyClient(calls) });
  assert.ok(!buttonByText('Adjust retrieval'), 'no button without a retrievalSettings node');
});

test('askAi records the query as a recent search, closes the palette and opens a new chat seeded with it', async () => {
  const calls = { agents: 0 };
  const { spies } = await driveToEmptyState({ searchClient: emptyClient(calls) });
  await click(buttonByText('Ask the AI directly')!);
  assert.deepEqual(spies.onSearch, ['zzz-nothing-matches'], 'query pushed to recents');
  assert.equal(spies.onClose.length, 1, 'palette closed');
  assert.deepEqual(spies.onNavigate, ['/platform/creatChat?q=zzz-nothing-matches'], 'new chat session opened with the query');
});

test('a custom onAskAi handler replaces the default navigation (shell-owned startChat)', async () => {
  const calls = { agents: 0 };
  const { spies } = await driveToEmptyState({
    searchClient: emptyClient(calls),
    onAskAi: (query: string) => { spies.onAskAi.push(query); },
  });
  await click(buttonByText('Ask the AI directly')!);
  assert.deepEqual(spies.onAskAi, ['zzz-nothing-matches']);
  assert.deepEqual(spies.onNavigate, [], 'default creatChat navigation not fired when onAskAi is provided');
});

test('adjustRetrieval opens the layered retrieval-settings drawer; overlay click closes it, palette stays open', async () => {
  const calls = { agents: 0 };
  const { spies } = await driveToEmptyState({
    searchClient: emptyClient(calls),
    retrievalSettings: <p data-testid="retrieval-body">retrieval form</p>,
  });
  await click(buttonByText('Adjust retrieval')!);
  const drawer = document.querySelector('[data-testid="cmdk-retrieval-drawer"]');
  assert.ok(drawer, 'drawer mounted above the palette');
  assert.ok((drawer!.textContent ?? '').includes('retrieval form'), 'shell-provided retrieval settings rendered inside the drawer');
  assert.ok((drawer!.textContent ?? '').includes('Search Settings'), 'drawer header uses retrievalSettings.title (en-US "Search Settings")');
  assert.equal(spies.onClose.length, 0, 'palette itself stays open');
  const overlay = document.querySelector('[data-testid="cmdk-retrieval-overlay"]');
  assert.ok(overlay, 'drawer has its own overlay');
  await act(async () => { overlay!.dispatchEvent(new window.MouseEvent('mousedown', { bubbles: true, cancelable: true })); });
  assert.ok(!document.querySelector('[data-testid="cmdk-retrieval-drawer"]'), 'overlay click closes the drawer');
  assert.equal(spies.onClose.length, 0, 'closing the drawer never closes the palette');
});

test('Escape closes the retrieval drawer first; a second Escape closes the palette', async () => {
  const calls = { agents: 0 };
  const { spies } = await driveToEmptyState({
    searchClient: emptyClient(calls),
    retrievalSettings: <p>retrieval form</p>,
  });
  await click(buttonByText('Adjust retrieval')!);
  await act(async () => {
    document.querySelector('[role="dialog"]')!.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
  });
  assert.ok(!document.querySelector('[data-testid="cmdk-retrieval-drawer"]'), 'first Escape dismisses the drawer');
  assert.equal(spies.onClose.length, 0, 'palette still open');
  await act(async () => {
    document.querySelector('[role="dialog"]')!.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
  });
  assert.equal(spies.onClose.length, 1, 'second Escape closes the palette');
});

// ─── agentsEnabled deployment gating ───

test('access.canOpenAgents=false hides the open-agents quick action from the idle palette', async () => {
  await mountPalette({ access: { canOpenAgents: false, canOpenOrganizations: true } });
  const text = document.body.textContent ?? '';
  assert.ok(!text.includes('Open agents'), 'agents quick action hidden');
  assert.ok(text.includes('Open shared spaces'), 'organizations quick action unaffected');
});

test('access.canOpenOrganizations=false hides the open-organizations quick action', async () => {
  await mountPalette({ access: { canOpenAgents: true, canOpenOrganizations: false } });
  const text = document.body.textContent ?? '';
  assert.ok(!text.includes('Open shared spaces'), 'organizations quick action hidden');
  assert.ok(text.includes('Open agents'), 'agents quick action unaffected');
});

test('agentsEnabled=false skips the agent fan-out entirely during live search', async () => {
  const calls = { agents: 0 };
  await driveToEmptyState({ searchClient: emptyClient(calls), agentsEnabled: false });
  assert.equal(calls.agents, 0, 'configuration.agents.list never called while the capability is off');
});

test('agentsEnabled=true (default) keeps loading the agent list for name matching', async () => {
  const calls = { agents: 0 };
  await driveToEmptyState({ searchClient: emptyClient(calls) });
  assert.ok(calls.agents >= 1, 'agent list loaded once for name matching');
});
