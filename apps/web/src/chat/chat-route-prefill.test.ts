import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';
import { build } from 'esbuild';

/*
 * R466-A2 ?q= prefill consumption (Vue menuStore.prefillQuery equivalent).
 *
 * Vue contract (frontend/src/components/Input-field.vue onMounted):
 *   const prefill = menuStore.consumePrefillQuery();
 *   if (prefill) { query.value = prefill; nextTick(() => textarea.focus()); }
 * — fill the composer, focus it, NEVER auto-send. The React deep link is
 * /platform/creatChat?q=… (R465 palette onAskAi fallback); consuming it must
 * prefill the captured ChatPage draft + focus signal, and strip the param
 * from the URL via replaceState once the query is sent or cleared.
 */

interface CapturedChatRouteProps {
  draft: string;
  composerFocusSignal: number;
  send(submission: { content: string; status: 'pending' }): Promise<void>;
  onDraftChange(value: string): void;
}

async function renderChatRoutePage(input: { location: string }): Promise<CapturedChatRouteProps & { historyCalls(): string[] }> {
  const historyCalls: string[] = [];
  const result = await build({
    stdin: {
      contents: `
        import React from 'react';
        import { renderToStaticMarkup } from 'react-dom/server';
        import { ChatRoutePage } from './ChatRoutePage.tsx';

        export function captureChatRoutePageProps(props) {
          renderToStaticMarkup(React.createElement(ChatRoutePage, props));
          return globalThis.__weknoraCapturedChatRouteProps;
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
      name: 'chat-route-prefill-test-stubs',
      setup(build) {
        build.onResolve({ filter: /^@weknora\/views(?:\/.*)?$/ }, () => ({ path: 'views-stub', namespace: 'test-stub' }));
        build.onLoad({ filter: /^views-stub$/, namespace: 'test-stub' }, () => ({
          loader: 'js',
          contents: `
            export function ChatPage(props) {
              globalThis.__weknoraCapturedChatRouteProps = props;
              return null;
            }
            export function openContextualGuide() {
              return false;
            }
            export function resolveChatCopy() {
              return { deleteConfirmBody: 'Delete this conversation?', streamFailed: '流式连接失败' };
            }
            export function splitLiveThinking() {
              return { showThink: false, thinking: false, thinkContent: '', answer: '' };
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
  const storage = new Map<string, string>();
  Object.assign(globalThis, {
    __weknoraCapturedChatRouteProps: undefined,
    localStorage: {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => { storage.set(key, value); },
      removeItem: (key: string) => { storage.delete(key); },
    },
    window: {
      location: input.location,
      history: {
        pushState: (_data: unknown, _unused: string, url?: string) => { historyCalls.push(`push:${String(url)}`); },
        replaceState: (_data: unknown, _unused: string, url?: string) => { historyCalls.push(`replace:${String(url)}`); },
      },
      open: () => null,
      setTimeout,
    },
  });
  const code = result.outputFiles[0]?.text;
  if (!code) throw new Error('ChatRoutePage prefill test bundle was empty');
  const module = { exports: {} as { captureChatRoutePageProps(props: Record<string, unknown>): CapturedChatRouteProps } };
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
    filename: 'chat-route-prefill-test-bundle.cjs',
  });
  const captured = module.exports.captureChatRoutePageProps({ client: chatClientStub(), scopeController: scopeControllerStub() });
  return { ...captured, historyCalls: () => historyCalls };
}

function scopeControllerStub() {
  return {
    current: () => ({ scope: { tenantId: 'tenant-1' }, signal: undefined }),
    isCurrent: () => true,
  };
}

function chatClientStub() {
  return {
    sessions: {
      create: async ({ title }: { title: string }) => ({ id: 'session-prefill', title, is_pinned: false }),
      messages: async () => [],
    },
    configuration: {
      agents: { listWithState: async () => ({ items: [], disabledOwnAgentIds: [] }) },
      mcp: { oauth: {} },
    },
    chat: {
      stream: async () => undefined,
      suggestions: { get: async () => ({ id: 'suggestions-1', questions: [] }), recordEvent: async () => undefined },
      approvals: { resolveTool: async () => undefined, cancelOAuth: async () => undefined, resolveOAuth: async () => undefined },
      artifacts: { message: async () => [], download: async () => ({ body: new Blob(), contentType: 'application/octet-stream' }) },
      steer: { enqueue: async () => undefined },
      stop: async () => undefined,
    },
  };
}

test('?q= prefills the composer draft and focuses it without auto-sending', async () => {
  const props = await renderChatRoutePage({ location: 'http://weknora.test/platform/creatChat?q=%E9%97%AE%20AI%20%E6%B5%8B%E8%AF%95' });
  // Vue Input-field.vue: query.value = prefill (fill only, never auto-send).
  assert.equal(props.draft, '问 AI 测试');
  // Vue nextTick(() => textarea.focus()) — the composer receives a focus signal.
  assert.equal(props.composerFocusSignal, 1);
  // No auto-send: rendering the entry point issues no history writes at all.
  assert.deepEqual(props.historyCalls(), []);
});

test('the new-chat entry without ?q= keeps an empty draft and no focus signal', async () => {
  const props = await renderChatRoutePage({ location: 'http://weknora.test/platform/creatChat' });
  assert.equal(props.draft, '');
  assert.equal(props.composerFocusSignal, 0);
});

test('a session route ignores a stray ?q= prefill (new-chat entry only)', async () => {
  const props = await renderChatRoutePage({ location: 'http://weknora.test/platform/chat/session-9?q=hi' });
  assert.equal(props.draft, '');
  assert.equal(props.composerFocusSignal, 0);
});

test('clearing the prefilled draft strips ?q= from the URL via replaceState', async () => {
  const props = await renderChatRoutePage({ location: 'http://weknora.test/platform/creatChat?q=hi' });
  props.onDraftChange('editing');
  props.onDraftChange('');
  assert.deepEqual(props.historyCalls(), ['replace:/platform/creatChat']);
});

test('sending strips ?q= from the URL via replaceState while keeping sibling params', async () => {
  const props = await renderChatRoutePage({ location: 'http://weknora.test/platform/creatChat?q=hi&agentId=agent-1' });
  await props.send({ content: 'hi', status: 'pending' });
  const history = props.historyCalls();
  assert.ok(history.includes('replace:/platform/creatChat?agentId=agent-1'), `replaceState calls: ${JSON.stringify(history)}`);
  // replaceState (not pushState) keeps the history stack clean.
  assert.ok(history.every((call) => call.startsWith('replace:') || call.startsWith('push:/platform/chat/')), JSON.stringify(history));
});
