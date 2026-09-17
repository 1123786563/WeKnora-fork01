// R447 A1 round: fixes for the R446 browser-evidence defects on the
// knowledge settings surface.
// 1. D2 (high): "测试分块效果" preview failed with "Invalid chunking preview
//    response" — the backend (internal/handler/chunker_debug.go) wraps the
//    preview payload in a {success: true, data: ...} envelope, the api-client
//    parser looked for selected_tier/tier_chain/... at the top level. The
//    contract test below drives createKnowledgeSettingsApi against the exact
//    backend JSON shape (path, method, request body, envelope response).
// 2. D3: the chunking-strategy select rendered a blank <option value="">.
//    Vue's wk-select shows a placeholder instead
//    (knowledgeEditor.chunking.strategyPlaceholder, already present in all
//    five locales) — the empty option must carry that label.
// 3. D5: with activity data present the section still rendered the "No
//    activity yet" summary card above the populated audit table (Vue
//    KnowledgeBaseActivitySettings.vue has no such overview card; its empty
//    state lives inside the table and is exclusive with rows). The summary
//    card's empty state must not render for the activity section.
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
const { KnowledgeSettingsPage } = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;
const { createKnowledgeSettingsApi } = await import('@weknora/api-client');

const knowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
  chunking_config: {
    chunk_size: 700,
    chunk_overlap: 90,
    separators: ['\n\n', '\n'],
    enable_parent_child: false,
    strategy: '',
    token_limit: 0,
    languages: [],
  },
};

// ---- D2: chunker/preview response-shape contract -----------------------------

// Mirrors internal/handler/chunker_debug.go PreviewChunkingResponse wrapped in
// the gin.H{"success": true, "data": ...} envelope the handler always returns.
const backendPreviewEnvelope = {
  success: true,
  data: {
    selected_tier: 'heading',
    tier_chain: ['heading', 'heuristic', 'legacy'],
    rejected: [{ tier: 'heuristic', reason: 'no structure match' }],
    profile: { total_lines: 4, total_chars: 120, md_heading_total: 1, form_feed_count: 0, german_chapter_count: 0, english_chapter_count: 0, chinese_chapter_count: 0, detected_langs: ['en'] },
    chunks: [{ seq: 1, start: 0, end: 120, size_chars: 120, size_tokens_approx: 40, context_header: 'Intro', content: 'hello chunk' }],
    stats: { count: 1, avg_chars: 120, min_chars: 120, max_chars: 120, stddev_chars: 0 },
  },
};

test('previewChunking parses the backend {success, data} envelope from POST /api/v1/chunker/preview', async () => {
  const seen: Array<{ method: string; path: string; body: unknown }> = [];
  const api = createKnowledgeSettingsApi(async (input: { method: string; path: string; body?: unknown }) => {
    seen.push({ method: input.method, path: input.path, body: input.body });
    return backendPreviewEnvelope;
  });
  const payload = {
    text: 'hello chunker',
    chunking_config: {
      chunk_size: 700,
      chunk_overlap: 90,
      separators: ['\n\n'],
      enable_parent_child: false,
      parent_chunk_size: 4096,
      child_chunk_size: 384,
      strategy: '',
      token_limit: 0,
      languages: [] as string[],
    },
  };
  const result = await api.previewChunking(payload);

  // Request contract (Vue frontend/src/api/chunker/index.ts: same path+verb).
  assert.equal(seen.length, 1);
  assert.equal(seen[0]!.method, 'POST');
  assert.equal(seen[0]!.path, '/api/v1/chunker/preview');
  assert.deepEqual(seen[0]!.body, payload);

  // Response contract: the envelope's data payload surfaces with the same
  // fields the Vue debug panel reads (frontend/src/types/chunker.ts).
  assert.equal(result.selected_tier, 'heading');
  assert.deepEqual(result.tier_chain, ['heading', 'heuristic', 'legacy']);
  assert.equal(result.chunks.length, 1);
  assert.equal(result.chunks[0]!.size_chars, 120);
  assert.equal(result.stats.count, 1);
  assert.ok(result.profile !== null && typeof result.profile === 'object');
  assert.equal((result.profile as Record<string, unknown>).md_heading_total, 1);
});

test('previewChunking rejects a non-envelope payload lacking the data wrapper', async () => {
  const api = createKnowledgeSettingsApi(async () => ({ success: true }));
  await assert.rejects(api.previewChunking({ text: 'x', chunking_config: { chunk_size: 100 } }), /Invalid chunking preview/);
});

// ---- D3: strategy select placeholder -----------------------------------------

interface UiCalls {
  requests: Array<{ method: string; path: string; body: Record<string, unknown> }>;
  activityCalls: Array<Record<string, unknown>>;
}

function clientFor(calls: UiCalls, options: { activityData?: Array<Record<string, unknown>> } = {}): WeKnoraClient {
  const request = async (input: { method: string; path: string; body: Record<string, unknown> }) => {
    calls.requests.push({ method: input.method, path: input.path, body: input.body });
    return { success: true };
  };
  return {
    request,
    configuration: { models: { list: async () => [] } },
    knowledgeBases: {
      documents: {
        list: async (kbId: string, params: Record<string, number> = {}) => {
          const query = new URLSearchParams(Object.entries(params).map(([key, value]) => [key, String(value)]));
          const suffix = query.toString();
          await request({ method: 'GET', path: `/api/v1/knowledge-bases/${encodeURIComponent(kbId)}/knowledge${suffix ? `?${suffix}` : ''}`, body: {} });
          return { data: [], total: 0 };
        },
      },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
        activity: async (_kbId: string, query: Record<string, unknown> = {}) => {
          calls.activityCalls.push(query);
          return { success: true, data: options.activityData ?? [], ...(options.activityData && options.activityData.length > 2 ? { next_cursor: 3 } : {}) };
        },
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient, kb: KnowledgeSettingsInput = knowledgeBase): Promise<void> {
  if (mountedRoot) {
    const previous = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { previous.unmount(); });
    document.body.innerHTML = '';
  }
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase: kb, role: 'admin' }));
  });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

async function openSection(section: string): Promise<void> {
  const button = document.body.querySelector(`button[data-section="${section}"]`);
  assert.ok(button, `expected a ${section} section button`);
  await act(async () => { button!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

test('the strategy select shows the Vue placeholder on its empty option instead of a blank row', async () => {
  const calls: UiCalls = { requests: [], activityCalls: [] };
  await renderPage(clientFor(calls));
  await openSection('chunking');

  const select = [...document.body.querySelectorAll<HTMLSelectElement>('select')].find((candidate) => candidate.getAttribute('aria-label') === 'Chunking Strategy');
  assert.ok(select, 'expected the chunking strategy select');
  const emptyOption = [...select.options].find((option) => option.value === '');
  assert.ok(emptyOption, 'expected the not-set empty option');
  assert.equal(
    (emptyOption.textContent ?? '').trim(),
    'Select a chunking strategy (splits by length if left empty)',
    `the empty option must carry the Vue placeholder text, got: ${JSON.stringify(emptyOption.textContent)}`,
  );
});

// ---- D5: activity summary card must not render its empty state ---------------

test('the activity section hides the "No activity yet" summary card while the audit table has rows', async () => {
  const calls: UiCalls = { requests: [], activityCalls: [] };
  const client = clientFor(calls, {
    activityData: [
      { id: 1, action: 'kb.updated', outcome: 'success', created_at: '2026-09-17T10:00:00Z' },
      { id: 2, action: 'knowledge.added', outcome: 'accepted', created_at: '2026-09-17T09:00:00Z' },
    ],
  });
  await renderPage(client);
  await openSection('activity');
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });

  // The audit table rendered its rows (the panel itself is exclusive between
  // its empty state and rows).
  const rows = [...document.body.querySelectorAll('table tbody tr')];
  assert.ok(rows.length >= 2, `expected the activity table rows to render, got ${rows.length}`);

  // R446 D5: the summary card above the panel still claimed "No activity
  // yet" — that empty state must be gone when data exists (Vue parity: the
  // activity section carries no overview card at all).
  const bodyText = document.body.textContent ?? '';
  assert.ok(!bodyText.includes('No activity yet'), 'the empty activity summary card must not coexist with table rows');
});
