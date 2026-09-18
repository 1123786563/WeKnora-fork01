import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';
import { build } from 'esbuild';

// Vue parity pin (Input-field.vue handleModelChange): localStorage records the
// user's *explicit* chat-model pick only. The composer's onModelChange is the
// single write site; the loader-seeded first-model fallback must never reach
// storage (the R476 regression where a synthetic default became durable).

async function renderChatRoutePage(input: {
  client: Record<string, unknown>;
  scopeController: Record<string, unknown>;
  location: string;
}): Promise<{ onModelChange: (modelId: string) => void; storage: Map<string, string>; storageWrites: string[] }> {
  const result = await build({
    stdin: {
      contents: `
        import React from 'react';
        import { renderToStaticMarkup } from 'react-dom/server';
        import { ChatRoutePage } from './ChatRoutePage.tsx';

        export function captureChatRoutePageProps(props) {
          renderToStaticMarkup(React.createElement(ChatRoutePage, props));
          return globalThis.__weknoraCapturedChatPageProps;
        }
      `,
      resolveDir: new URL('.', import.meta.url).pathname,
      loader: 'ts',
    },
    bundle: true,
    format: 'cjs',
    platform: 'node',
    jsx: 'automatic',
    write: false,
    plugins: [{
      name: 'chat-route-page-model-pick-stubs',
      setup(build) {
        build.onResolve({ filter: /^@weknora\/views(?:\/.*)?$/ }, () => ({ path: 'views-stub', namespace: 'test-stub' }));
        build.onLoad({ filter: /^views-stub$/, namespace: 'test-stub' }, () => ({
          loader: 'js',
          contents: `
            export function ChatPage(props) {
              globalThis.__weknoraCapturedChatPageProps = props;
              return null;
            }
            export function openContextualGuide() {
              return false;
            }
            export function resolveChatCopy() {
              return { streamFailed: '流式连接失败' };
            }
            export function splitLiveThinking(content) {
              return { showThink: false, thinking: false, thinkContent: '', answer: content ?? '' };
            }
            export function resolveForkAffordance() {
              return { canFork: false };
            }
            export function stashForkLanding() {}
            export function takeForkLanding() {
              return null;
            }
          `,
        }));
        build.onLoad({ filter: /\.css$/ }, () => ({ loader: 'js', contents: '' }));
      },
    }],
  });
  const href = new URL(input.location);
  const storage = new Map<string, string>();
  const storageWrites: string[] = [];
  const storageStub = {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => { storageWrites.push(key); storage.set(key, value); },
    removeItem: (key: string) => { storage.delete(key); },
  };
  Object.assign(globalThis, {
    localStorage: storageStub,
    window: {
      location: href,
      history: { pushState: () => undefined, replaceState: () => undefined },
      open: () => null,
      setTimeout,
      localStorage: storageStub,
    },
  });
  const code = result.outputFiles[0]?.text;
  if (!code) throw new Error('ChatRoutePage test bundle was empty');
  const module = { exports: {} as {
    captureChatRoutePageProps(props: Record<string, unknown>): Record<string, unknown>;
  } };
  vm.runInNewContext(code, {
    module,
    exports: module.exports,
    require: createRequire(import.meta.url),
    globalThis,
    window: (globalThis as typeof globalThis & { window: unknown }).window,
    localStorage: (globalThis as typeof globalThis & { localStorage: unknown }).localStorage,
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
  }, {
    filename: 'chat-route-page-model-pick-test-bundle.cjs',
  });
  const props = module.exports.captureChatRoutePageProps({
    client: input.client,
    scopeController: input.scopeController,
  });
  return {
    onModelChange: props.onModelChange as (modelId: string) => void,
    storage,
    storageWrites,
  };
}

const scopeController = {
  current: () => ({ scope: { tenantId: 'tenant-1' }, signal: undefined }),
  isCurrent: () => true,
};

const client = {
  configuration: {
    models: { list: async () => [{ id: 'm1', type: 'KnowledgeQA', name: 'Model One' }] },
    agents: { listWithState: async () => ({ items: [], disabledOwnAgentIds: [] }) },
    mcp: { oauth: {} },
  },
  sessions: {
    list: async () => ({ items: [], total: 0 }),
    messages: async () => [],
  },
  knowledge: { knowledgeBases: { list: async () => ({ items: [] }) } },
  chat: { suggestions: { get: async () => undefined } },
};

test('an explicit composer model pick is persisted to the per-scope storage key', async () => {
  const storageKey = 'weknora:last-chat-model:undefined:anonymous:tenant-1';
  const captured = await renderChatRoutePage({
    client,
    scopeController,
    location: 'http://localhost:5175/platform/creatChat',
  });
  captured.onModelChange('m2');
  assert.deepEqual(
    captured.storageWrites,
    [storageKey],
    `model pick write must land on exactly the per-scope key (writes: ${JSON.stringify(captured.storageWrites)})`,
  );
  assert.equal(captured.storage.get(storageKey), 'm2');
});

test('a bare render never writes a model pick (no synthetic first-model persistence)', async () => {
  const captured = await renderChatRoutePage({
    client,
    scopeController,
    location: 'http://localhost:5175/platform/creatChat',
  });
  assert.equal(typeof captured.onModelChange, 'function');
  assert.deepEqual(captured.storageWrites.filter((key) => key.includes('last-chat-model')), []);
});
