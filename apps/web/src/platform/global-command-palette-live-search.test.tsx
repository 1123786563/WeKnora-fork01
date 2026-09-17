// R464-A1 — live knowledge search in the React GlobalCommandPalette. Vue
// contract (frontend/src/components/GlobalCommandPalette.vue + useSearch.ts):
//   • 350ms debounce, non-empty trimmed query fans out to
//     POST /api/v1/knowledge-search (chunks) + /api/v1/messages/search, plus
//     client-side KB/agent name matches and keyword session-title matches.
//   • Group order with a query: chunks(≤5) → messages(≤4) → kbs → agents →
//     sessions → commands. Scoped to one KB: chunks only.
//   • KB scope chip: seeded from the KB detail route, placeholder switches,
//     Backspace on empty input (or ✕) clears it and re-runs the search.
//   • Chunk click → /platform/knowledge-bases/{kbId}?knowledge_id={id};
//     message/session click → /platform/chat/{sessionId}; kb click →
//     /platform/knowledge-bases/{kbId}.
//   • Empty state copy appears only after a settled search with no hits.
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
import type { GlobalCommandPaletteProps } from './GlobalCommandPalette.tsx';

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

const CHUNK_HIT = {
  id: 'ch-1',
  content: 'full body text',
  matchedContent: '…Q3 revenue grew 12%…',
  knowledgeId: 'doc-1',
  knowledgeBaseId: 'kb-1',
  knowledgeTitle: 'Revenue Report',
  knowledgeFilename: 'rev.pdf',
  chunkIndex: 2,
  score: 0.87,
  matchType: 'vector',
};

interface ClientCalls {
  chunkSearch: Array<{ query: string; knowledgeBaseIds: readonly string[] }>;
  kbList: number;
  messageSearch: Array<{ query: string }>;
  sessions: number;
  agents: number;
}

function makeClient(options: {
  chunks?: unknown[];
  messages?: Array<Record<string, unknown>>;
  kbs?: Array<{ id: string; name: string }>;
  sessions?: Array<{ id: string; title: string }>;
  agents?: Array<{ id: string; name: string; description: string }>;
} = {}): { client: object; calls: ClientCalls } {
  const calls: ClientCalls = { chunkSearch: [], kbList: 0, messageSearch: [], sessions: 0, agents: 0 };
  const kbs = options.kbs ?? [{ id: 'kb-1', name: 'Alpha KB' }, { id: 'kb-2', name: 'Beta KB' }];
  const client = {
    knowledgeBases: {
      list: async () => { calls.kbList += 1; return kbs; },
      search: async (params: { query: string; knowledgeBaseIds: readonly string[] }) => {
        calls.chunkSearch.push(params);
        return options.chunks ?? [CHUNK_HIT];
      },
    },
    settings: {
      chatHistory: {
        search: async (input: { query: string }) => {
          calls.messageSearch.push(input);
          return { items: options.messages ?? [], total: (options.messages ?? []).length };
        },
      },
    },
    sessions: {
      list: async () => { calls.sessions += 1; return { data: options.sessions ?? [] }; },
    },
    configuration: {
      agents: {
        list: async () => { calls.agents += 1; return options.agents ?? []; },
      },
    },
  };
  return { client, calls };
}

interface MountOptions {
  client?: object;
  initialKbScope?: { id: string; name: string } | null;
}

async function mountPalette(options: MountOptions = {}): Promise<Record<string, { calls: unknown[] }>> {
  const spies = {
    onClose: { calls: [] as unknown[] },
    onNavigate: { calls: [] as unknown[] },
    onSearch: { calls: [] as unknown[] },
    onClearRecent: { calls: [] as unknown[] },
  };
  const props = {
    open: true,
    initialQuery: '',
    recentQueries: [],
    locale: 'en-US' as const,
    onClose: () => { spies.onClose.calls.push(true); },
    onNavigate: (path: string) => { spies.onNavigate.calls.push(path); },
    onSearch: (query: string) => { spies.onSearch.calls.push(query); },
    onClearRecent: () => { spies.onClearRecent.calls.push(true); },
    ...(options.client !== undefined ? { searchClient: options.client as GlobalCommandPaletteProps['searchClient'] } : {}),
    ...(options.initialKbScope !== undefined ? { initialKbScope: options.initialKbScope } : {}),
    searchDebounceMs: 10,
  };
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(GlobalCommandPalette, props));
  });
  await settle(5);
  return spies;
}

const input = () => document.querySelector<HTMLInputElement>('.cmdk__input');

const typeQuery = (value: string) => act(async () => {
  const el = input();
  if (!el) throw new Error('palette input not mounted');
  // React dedupes plain `.value =` writes; use the native prototype setter
  // (same approach as OrganizationsPage.test.tsx setInputValue).
  const setter = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  setter?.call(el, value);
  el.dispatchEvent(new window.Event('input', { bubbles: true }));
});

const pressKey = (init: { key: string; metaKey?: boolean; ctrlKey?: boolean }) =>
  act(async () => {
    input()?.dispatchEvent(new window.KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }));
  });

test('typing a query fires one debounced knowledge-search over all visible KBs (and message search)', async () => {
  const { client, calls } = makeClient();
  await mountPalette({ client });
  await typeQuery('revenue');
  assert.equal(calls.chunkSearch.length, 0, 'no request before the debounce elapses');
  await settle(30);
  assert.equal(calls.chunkSearch.length, 1, 'exactly one knowledge-search after the debounce');
  assert.equal(calls.chunkSearch[0]!.query, 'revenue');
  assert.deepEqual(calls.chunkSearch[0]!.knowledgeBaseIds, ['kb-1', 'kb-2'], 'unscoped search fans out to every visible KB');
  assert.equal(calls.messageSearch.length, 1, 'message search rides along when unscoped');
  assert.equal(calls.kbList, 1, 'KB list is loaded once to resolve the scope');
});

test('chunk hits render as grouped file cards with match badge, KB chip and matched excerpt', async () => {
  const { client } = makeClient({
    chunks: [CHUNK_HIT],
    messages: [{ request_id: 'r-1', session_id: 's-1', session_title: 'Q3 review', query_content: 'revenue?', answer_content: 'grew', score: 0.5, match_type: 'keyword', created_at: '' }],
  });
  await mountPalette({ client });
  // 'alpha' matches both the mocked chunk search (mock ignores the query)
  // and the KB named "Alpha KB", so the KB name-match group renders too.
  await typeQuery('alpha');
  await settle(30);
  const text = document.body.textContent ?? '';
  assert.ok(text.includes('Files'), 'chunk group label');
  assert.ok(text.includes('Revenue Report'), 'chunk card title');
  assert.ok(text.includes('Alpha KB'), 'KB name chip on the card');
  assert.ok(text.includes('Q3 revenue grew 12%'), 'matched excerpt as the card subtitle');
  assert.ok(text.includes('Vector'), 'match-type badge');
  assert.ok(text.includes('Messages'), 'message group label');
  assert.ok(text.includes('Q3 review'), 'message card title');
  assert.ok(text.includes('Knowledge bases'), 'KB name-match group label (Alpha KB matches)');
});

test('KB scope narrows the search to one KB, switches the placeholder, and disables message search', async () => {
  const { client, calls } = makeClient();
  await mountPalette({ client, initialKbScope: { id: 'kb-2', name: 'Beta KB' } });
  assert.equal(input()?.getAttribute('placeholder'), 'Search within this knowledge base…', 'scoped placeholder');
  assert.ok((document.body.textContent ?? '').includes('Beta KB'), 'scope chip shows the KB name');
  await typeQuery('revenue');
  await settle(30);
  assert.equal(calls.chunkSearch.length, 1);
  assert.deepEqual(calls.chunkSearch[0]!.knowledgeBaseIds, ['kb-2'], 'scoped search targets only the locked KB');
  assert.equal(calls.messageSearch.length, 0, 'message search is disabled while scoped');
  const text = document.body.textContent ?? '';
  assert.ok(text.includes('Files'), 'scoped group order shows chunks');
  assert.ok(!text.includes('Messages'), 'no message group while scoped');
  assert.ok(!text.includes('Commands'), 'Vue scoped groupOrder is chunks-only');
});

test('clearing the scope chip re-runs the search across all KBs and restores the global placeholder', async () => {
  const { client, calls } = makeClient();
  await mountPalette({ client, initialKbScope: { id: 'kb-2', name: 'Beta KB' } });
  await typeQuery('revenue');
  await settle(30);
  const chipX = document.querySelector('[data-cmdk-scope-remove]');
  assert.ok(chipX, 'scope chip has a remove button');
  await act(async () => { chipX!.dispatchEvent(new window.MouseEvent('click', { bubbles: true })); });
  await settle(30);
  assert.equal(input()?.getAttribute('placeholder'), 'Search knowledge bases, files, conversations…');
  assert.equal(calls.chunkSearch.length, 2, 'search re-ran after the scope changed');
  assert.deepEqual(calls.chunkSearch[1]!.knowledgeBaseIds, ['kb-1', 'kb-2']);
});

test('clicking a chunk navigates to the KB document and records the search', async () => {
  const { client } = makeClient({ chunks: [CHUNK_HIT] });
  const spies = await mountPalette({ client });
  await typeQuery('revenue');
  await settle(30);
  const card = [...document.querySelectorAll('[data-cmdk-index]')].find((el) => (el.textContent ?? '').includes('Revenue Report'));
  assert.ok(card, 'chunk card rendered');
  await act(async () => { card!.dispatchEvent(new window.MouseEvent('click', { bubbles: true })); });
  assert.deepEqual(spies.onNavigate.calls, ['/platform/knowledge-bases/kb-1?knowledge_id=doc-1']);
  assert.equal(spies.onClose.calls.length, 1, 'palette closes after opening a chunk');
  assert.deepEqual(spies.onSearch.calls, ['revenue'], 'the query lands in recent searches');
});

test('keyboard arrows traverse chunks then commands; Enter opens the selection', async () => {
  // No message/session/agent hits and a query that keeps the new-chat command
  // in the filtered list → flat order is [chunk, new-chat, …].
  const { client } = makeClient({ chunks: [CHUNK_HIT], messages: [], sessions: [], agents: [] });
  const spies = await mountPalette({ client });
  await typeQuery('chat');
  await settle(30);
  await pressKey({ key: 'ArrowDown' });
  await pressKey({ key: 'Enter' });
  assert.deepEqual(spies.onNavigate.calls, ['/platform/creatChat'], 'ArrowDown moves selection onto the first command; Enter runs it');
  assert.equal(spies.onClose.calls.length, 1);
});

test('a settled search with no hits anywhere shows the localized empty state', async () => {
  const { client } = makeClient({ chunks: [], messages: [] });
  await mountPalette({ client });
  await typeQuery('zzz-nothing-matches');
  await settle(30);
  assert.ok((document.body.textContent ?? '').includes('No matches found'));
});
