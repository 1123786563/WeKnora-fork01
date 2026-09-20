import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';
import { build } from 'esbuild';

/**
 * R490 B (R489 D12) — integration test for the chat `@` mention popup scope.
 *
 * Mounts the REAL ChatRoutePage (real @weknora/views ChatPage/composer) in a
 * jsdom document with createRoot + act, so the mount effects (agent list
 * load) and the mention load pipeline actually run. The Vue popup scopes the
 * KB list through chatResources.validKnowledgeBases (the `isKbModelReady`
 * initialization filter: summary_model_id required, embedding_model_id when
 * chunk indexing) before the agent compatibility pass (Input-field.vue
 * 1288-1399), so a tenant whose KBs never finished model configuration shows
 * the popup empty state — the exact R489 D12 repro (Parity FAQ Fixture / Wiki
 * Parity Fixture / Parity KB Demo all lacking model ids).
 */

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/creatChat' });
Object.assign(globalThis, { window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const require_ = createRequire(import.meta.url);

interface BundleApi {
  mount(client: unknown, scopeController: unknown): Promise<HTMLElement>;
  act(fn: () => Promise<void> | void): Promise<void>;
  click(el: HTMLElement): Promise<void>;
  unmount(): Promise<void>;
}

let bundleModule: BundleModule;
type BundleModule = BundleApi | undefined;

async function loadBundle(): Promise<void> {
  if (bundleModule) return;
  const result = await build({
    stdin: {
      contents: `
        import React, { act as reactAct } from 'react';
        import { createRoot } from 'react-dom/client';
        import { ChatRoutePage } from './ChatRoutePage.tsx';

        export async function mount(client, scopeController) {
          const container = globalThis.document.createElement('div');
          globalThis.document.body.append(container);
          const root = createRoot(container);
          globalThis.__weknoraChatRoot = root;
          await reactAct(async () => {
            root.render(React.createElement(ChatRoutePage, { client, scopeController }));
          });
          return container;
        }

        export async function act(fn) {
          await reactAct(async () => { await fn(); });
        }

        export async function click(el) {
          await reactAct(async () => { el.click(); });
        }

        export async function unmount() {
          const root = globalThis.__weknoraChatRoot;
          if (root) await reactAct(async () => { root.unmount(); });
          globalThis.__weknoraChatRoot = undefined;
        }
      `,
      resolveDir: new URL('.', import.meta.url).pathname,
      loader: 'ts',
    },
    bundle: true,
    format: 'cjs',
    platform: 'browser',
    jsx: 'automatic',
    write: false,
    plugins: [{
      name: 'chat-route-mention-popup-test',
      setup(build) {
        // CSS modules are side-effect styles; jsdom does not need them.
        build.onLoad({ filter: /\.css$/ }, () => ({ loader: 'js', contents: '' }));
      },
    }],
  });
  const code = result.outputFiles[0]?.text;
  if (!code) throw new Error('ChatRoutePage mention-popup test bundle was empty');
  const module = { exports: {} as Record<string, unknown> };
  // React's scheduler uses MessageChannel ports for task queueing; Node ports
  // pin the event loop after the tests finish. A task-based stand-in keeps the
  // scheduler semantics without lingering handles.
  const taskMessageChannel = {
    MessageChannel: class MessageChannel {
      port1: { onmessage: ((event: { data: unknown }) => void) | null } = { onmessage: null };
      port2 = {
        postMessage: (message: unknown): void => {
          const port1 = this.port1;
          queueMicrotask(() => port1.onmessage?.({ data: message }));
        },
      };
    },
  };
  vm.runInNewContext(code, {
    module,
    exports: module.exports,
    require: require_,
    globalThis,
    window: dom.window,
    document: dom.window.document,
    navigator: dom.window.navigator,
    localStorage: dom.window.localStorage,
    HTMLElement: dom.window.HTMLElement,
    IS_REACT_ACT_ENVIRONMENT: true,
    process,
    console,
    setTimeout,
    clearTimeout,
    queueMicrotask,
    performance,
    URL,
    URLSearchParams,
    Blob,
    DOMException,
    AbortController,
    MessageChannel: taskMessageChannel.MessageChannel as unknown as typeof MessageChannel,
    CustomEvent: dom.window.CustomEvent,
    PopStateEvent: dom.window.PopStateEvent,
    Event: dom.window.Event,
    KeyboardEvent: dom.window.KeyboardEvent,
    MutationObserver: dom.window.MutationObserver,
    getComputedStyle: dom.window.getComputedStyle,
    requestAnimationFrame: dom.window.requestAnimationFrame,
    cancelAnimationFrame: dom.window.cancelAnimationFrame,
  }, { filename: 'chat-route-mention-popup-test-bundle.cjs' });
  bundleModule = module.exports as unknown as BundleApi;
}

// --- fixtures ----------------------------------------------------------------

/** R489 D12 repro: the parity tenant's KBs (live shapes, 2026-09-20). FAQ has
 *  a summary LLM but no embedding model (chunk-indexed → not ready); the
 *  other two never configured the summary LLM at all. */
const RAW_PARITY_KBS = [
  { id: 'kb-faq', name: 'Parity FAQ Fixture', type: 'faq', summary_model_id: 'builtin-llm-mock', embedding_model_id: '', indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: false }, capabilities: { faq: true, vector: true, keyword: true } },
  { id: 'kb-wiki', name: 'Wiki Parity Fixture', summary_model_id: '', embedding_model_id: '', indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true, graph_enabled: false }, capabilities: { wiki: true } },
  { id: 'kb-demo', name: 'Parity KB Demo', summary_model_id: '', embedding_model_id: '', indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: false }, capabilities: { vector: true, keyword: true } },
];

/** A KB that finished model configuration (chatResources.isKbModelReady). */
const READY_KB = { id: 'kb-ready', name: 'Ready KB', summary_model_id: 'llm-1', embedding_model_id: 'emb-1', capabilities: { vector: true } };

const PARITY_FILES = [
  { id: 'file-1', title: 'mermaid-arch-demo.md', knowledge_base_id: 'kb-demo', knowledge_base_name: 'Parity KB Demo' },
  { id: 'file-2', title: 'weknora-upload-test.md', knowledge_base_id: 'kb-faq', knowledge_base_name: 'Parity FAQ Fixture' },
];

const QUICK_ANSWER_AGENT = {
  id: 'builtin-quick-answer',
  name: '快速问答',
  description: '',
  is_builtin: true,
  // The live parity tenant config: no agent_mode, no kb_selection_mode, no
  // allowed_tools — the Vue compatibility filter stays open and ONLY the
  // initialization (isKbModelReady) filter empties the list.
  config: { model_id: 'builtin-llm-mock' },
};

function scopeControllerStub() {
  // `scope` feeds effect deps across the page — the object identities must be
  // stable or every render re-fires the sessions/models effects (mount loop).
  const scope = { tenantId: 'tenant-1' };
  return {
    current: () => ({ scope, signal: undefined }),
    isCurrent: () => true,
  };
}

interface MentionClientOverrides {
  kbs?: Array<Record<string, unknown>>;
  files?: Array<Record<string, unknown>>;
  shared?: Array<Record<string, unknown>>;
  agents?: Array<Record<string, unknown>>;
}

function mentionClientStub(overrides: MentionClientOverrides = {}) {
  const tagCalls: string[] = [];
  const client = {
    sessions: {
      list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }),
      messages: async () => [],
      create: async () => ({ id: 'session-1', title: '新对话', is_pinned: false }),
    },
    configuration: {
      agents: {
        listWithState: async () => ({ items: overrides.agents ?? [QUICK_ANSWER_AGENT], disabledOwnAgentIds: [] }),
      },
      models: { list: async () => [] },
      mcp: { list: async () => [] },
      skills: { list: async () => [] },
      oauth: {},
    },
    settings: {
      webSearch: { providers: { list: async () => [] } },
      preferences: { get: async () => ({}) },
    },
    knowledgeBases: {
      list: async () => overrides.kbs ?? [...RAW_PARITY_KBS],
    },
    knowledge: {
      documents: {
        search: async () => ({ data: overrides.files ?? [...PARITY_FILES] }),
        tags: async (kbId: string) => { tagCalls.push(kbId); return []; },
      },
    },
    identity: {
      organizations: {
        knowledgeBaseShares: { listShared: async () => overrides.shared ?? [] },
      },
    },
    chat: {
      suggestions: { get: async () => ({ id: 's-1', questions: [] }), recordEvent: async () => undefined },
      approvals: {},
      steer: {},
    },
  };
  return { client, tagCalls };
}

async function openMentionPopup(): Promise<HTMLElement> {
  await loadBundle();
  const { client } = mentionClientStub();
  const container = await bundleModule!.mount(client, scopeControllerStub());
  // Flush the mount effects (agents/models/sessions loads) through act.
  for (let i = 0; i < 4; i++) await bundleModule!.act(async () => { await Promise.resolve(); });
  const atButton = container.querySelector<HTMLElement>('[data-guide="chat-kb-mention"]');
  assert.ok(atButton, 'composer @ button rendered');
  await bundleModule!.click(atButton);
  // Flush the mention load pipeline (kb list + files + tags).
  for (let i = 0; i < 6; i++) await bundleModule!.act(async () => { await Promise.resolve(); });
  const popup = container.querySelector<HTMLElement>('#wk-chat-mention-listbox');
  assert.ok(popup, 'mention popup opens on @ button click');
  return container;
}

function mentionOptions(container: HTMLElement): Array<{ type: string; name: string }> {
  return [...container.querySelectorAll<HTMLElement>('[data-mention-id]')].map((el) => ({
    type: el.getAttribute('data-mention-type') ?? '',
    name: (el.textContent ?? '').replace(/^[@#▧]/, ''),
  }));
}

test.afterEach(async () => {
  if (bundleModule) await bundleModule.unmount();
  document.body.replaceChildren();
});

// --- D12: initialization filter (chatResources.isKbModelReady) ------------------

test('@ popup drops KBs without model configuration like Vue validKnowledgeBases (R489 D12 repro)', async () => {
  const container = await openMentionPopup();
  const options = mentionOptions(container);
  assert.deepEqual(options, [], 'KBs lacking summary/embedding model ids must not be @-mentionable, so the popup shows the empty state');
});

test('@ popup keeps a model-ready KB, scopes tags to it, and drops files from other KBs', async () => {
  await loadBundle();
  const { client, tagCalls } = mentionClientStub({
    kbs: [...RAW_PARITY_KBS, READY_KB],
    files: [
      ...PARITY_FILES,
      { id: 'file-3', title: 'ready-notes.md', knowledge_base_id: 'kb-ready', knowledge_base_name: 'Ready KB' },
    ],
  });
  const container = await bundleModule!.mount(client, scopeControllerStub());
  for (let i = 0; i < 4; i++) await bundleModule!.act(async () => { await Promise.resolve(); });
  const atButton = container.querySelector<HTMLElement>('[data-guide="chat-kb-mention"]');
  assert.ok(atButton);
  await bundleModule!.click(atButton);
  for (let i = 0; i < 6; i++) await bundleModule!.act(async () => { await Promise.resolve(); });

  const options = mentionOptions(container);
  assert.deepEqual(options, [{ type: 'kb', name: 'Ready KB' }, { type: 'file', name: 'ready-notes.md' }],
    'only the model-ready KB and its files survive the scope');
  assert.deepEqual(tagCalls, ['kb-ready'], 'KB tag enumeration only runs for the scoped KB');
});

test('@ popup merges writable-shared KBs after the own rows, deduped by id (Input-field.vue 1265-1284)', async () => {
  await loadBundle();
  const { client } = mentionClientStub({
    kbs: [READY_KB, { id: 'kb-shared-dupe', name: 'Own Dup', summary_model_id: 'llm-1', indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true } }],
    shared: [
      { knowledge_base: { id: 'kb-shared-1', name: 'Shared KB', capabilities: { vector: true } }, permission: 'editor', org_name: 'Parity Org' },
      { knowledge_base: { id: 'kb-shared-dupe', name: 'Own Dup (shared copy)' }, permission: 'admin', org_name: 'Parity Org' },
      { knowledge_base: null, permission: 'viewer' },
    ],
  });
  const container = await bundleModule!.mount(client, scopeControllerStub());
  for (let i = 0; i < 4; i++) await bundleModule!.act(async () => { await Promise.resolve(); });
  const atButton = container.querySelector<HTMLElement>('[data-guide="chat-kb-mention"]');
  assert.ok(atButton);
  await bundleModule!.click(atButton);
  for (let i = 0; i < 6; i++) await bundleModule!.act(async () => { await Promise.resolve(); });

  const options = mentionOptions(container);
  assert.deepEqual(options.map((o) => o.name), ['Ready KB', 'Own Dup', 'Shared KB'],
    'shared KBs append after own rows; the own row wins the id dedup; null knowledge_base rows are skipped');
});
