import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';
import { build } from 'esbuild';

interface CapturedChatPageProps {
  send(submission: { content: string; status: 'pending' }): Promise<void>;
  /** R471-A1: the host derives Vue isAgentStreamSession() parity here. */
  canSteer?: boolean;
}

async function renderChatRoutePage(input: {
  client: Record<string, unknown>;
  scopeController: Record<string, unknown>;
  location: string;
}): Promise<CapturedChatPageProps> {
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
      name: 'chat-route-page-test-stubs',
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
          return { deleteConfirmBody: 'Delete this conversation? This cannot be undone.', knowledgeBasesLoadFailed: 'Failed to load knowledge bases', streamFailed: '流式连接失败' };
        }
        export function splitLiveThinking(content) {
          if (!content || !content.includes('<think>')) {
            return { showThink: false, thinking: false, thinkContent: '', answer: content ?? '' };
          }
          if (!content.includes('</think>')) {
            return { showThink: true, thinking: true, thinkContent: content.replace('<think>', '').trim(), answer: '' };
          }
          const index = content.lastIndexOf('</think>');
          return { showThink: true, thinking: false, thinkContent: content.substring(0, index).replace('<think>', ''), answer: content.substring(index + 8).trim() };
        }
            export function resolveForkAffordance() {
              return { canFork: false };
            }
            export function getAgentNotReadyReasonKeys() {
              return [];
            }
            export function agentNotReadyLabels() {
              return [];
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
  Object.assign(globalThis, {
    localStorage: {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => { storage.set(key, value); },
      removeItem: (key: string) => { storage.delete(key); },
    },
    window: {
      location: href,
      history: {
        pushState: () => undefined,
        replaceState: () => undefined,
      },
      open: () => null,
      setTimeout,
    },
  });
  const code = result.outputFiles[0]?.text;
  if (!code) throw new Error('ChatRoutePage test bundle was empty');
  const module = { exports: {} as {
    captureChatRoutePageProps(props: Record<string, unknown>): CapturedChatPageProps;
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
    filename: 'chat-route-page-test-bundle.cjs',
  });
  return module.exports.captureChatRoutePageProps({
    client: input.client,
    scopeController: input.scopeController,
  });
}

test('ChatRoutePage first creatChat send opens the stream request after session selection with a live signal', async () => {
  const calls: string[] = [];
  const scopeController = {
    current: () => ({ scope: { tenantId: 'tenant-1' }, signal: undefined }),
    isCurrent: () => true,
  };
  const client = {
    sessions: {
      create: async ({ title }: { title: string }) => {
        calls.push(`create:${title}`);
        return { id: 'session-created', title, is_pinned: false };
      },
      messages: async (sessionId: string) => {
        calls.push(`messages:${sessionId}`);
        return [
          { id: 'user-1', session_id: sessionId, role: 'user', content: 'hello', is_completed: true },
          { id: 'assistant-1', session_id: sessionId, role: 'assistant', content: 'hi', is_completed: true },
        ];
      },
    },
    configuration: {
      agents: {
        listWithState: async () => ({ items: [], disabledOwnAgentIds: [] }),
      },
      mcp: { oauth: {} },
    },
    chat: {
      stream: async (options: { sessionId: string; signal?: AbortSignal }) => {
        if (options.signal?.aborted) {
          calls.push(`stream-suppressed:${options.sessionId}`);
          throw new DOMException('Aborted', 'AbortError');
        }
        calls.push(`stream-request:${options.sessionId}`);
      },
      suggestions: {
        get: async () => ({ id: 'suggestions-1', questions: [] }),
        recordEvent: async () => undefined,
      },
      approvals: {
        resolveTool: async () => undefined,
        cancelOAuth: async () => undefined,
        resolveOAuth: async () => undefined,
      },
      artifacts: {
        message: async () => [],
        download: async () => ({ body: new Blob(), contentType: 'application/octet-stream' }),
      },
      steer: { enqueue: async () => undefined },
      stop: async () => undefined,
    },
  };
  const props = await renderChatRoutePage({
    client,
    scopeController,
    location: 'http://weknora.test/platform/creatChat',
  });
  (globalThis.window as { history: { pushState: (...args: unknown[]) => void } }).history.pushState = (...args) => {
    calls.push(`select:${String(args[2])}`);
  };

  await props.send({ content: 'hello', status: 'pending' });

  assert.deepEqual(calls, [
    'create:hello',
    'select:/platform/chat/session-created',
    'stream-request:session-created',
    'messages:session-created',
  ]);
});

/*
 * Vue stream 404 copy (streame.ts onopen/onerror): a failed handshake rejects
 * with 「流式连接失败: HTTP 404」 — the localized prefix plus the HTTP status —
 * instead of a generic 「操作失败」. The pending row keeps its retry entry.
 */
/*
 * R471-A1 root cause 1: ChatRoutePage always wires onSteer (the steer()
 * helper with its idle fallback), and ChatPage used to derive canSteer from
 * Boolean(onSteer) — so every session, including the default quick-answer
 * chat, advertised steer capability and the stop button
 * (`isReplying && (!canSteer || !draft)`) could never win with a non-empty
 * draft. Vue computes canSteer from isAgentStreamSession(); the React
 * counterpart is the selected agent pipeline (buildWebChatStreamOptions
 * mode 'agent' ⇔ a selectedAgentId). Without an agent the page must report
 * canSteer=false.
 */
test('ChatRoutePage reports canSteer=false for the default quick-answer session despite wiring onSteer', async () => {
  const scopeController = {
    current: () => ({ scope: { tenantId: 'tenant-1' }, signal: undefined }),
    isCurrent: () => true,
  };
  const client = {
    sessions: { create: async () => ({ id: 'session-1', title: '', is_pinned: false }), messages: async () => [] },
    configuration: { agents: { listWithState: async () => ({ items: [], disabledOwnAgentIds: [] }) }, mcp: { oauth: {} } },
    chat: {
      stream: async () => undefined,
      suggestions: { get: async () => ({ id: 'suggestions-1', questions: [] }), recordEvent: async () => undefined },
      approvals: { resolveTool: async () => undefined, cancelOAuth: async () => undefined, resolveOAuth: async () => undefined },
      artifacts: { message: async () => [], download: async () => ({ body: new Blob(), contentType: 'application/octet-stream' }) },
      steer: { enqueue: async () => undefined },
      stop: async () => undefined,
    },
  };
  (globalThis.window as { history: { pushState: (...args: unknown[]) => void } }).history.pushState = () => undefined;
  const props = await renderChatRoutePage({
    client,
    scopeController,
    location: 'http://weknora.test/platform/creatChat',
  });
  assert.equal(props.canSteer, false, 'no selected agent means quick-answer: no steer capability');
});

test('a 404 stream handshake rejects with the Vue-localized stream failure message', async () => {
  const scopeController = {
    current: () => ({ scope: { tenantId: 'tenant-1' }, signal: undefined }),
    isCurrent: () => true,
  };
  const client = {
    sessions: {
      create: async ({ title }: { title: string }) => ({ id: 'session-404', title, is_pinned: false }),
      messages: async () => [],
    },
    configuration: {
      agents: { listWithState: async () => ({ items: [], disabledOwnAgentIds: [] }) },
      mcp: { oauth: {} },
    },
    chat: {
      // No event was received, so no Last-Event-ID resume is offered: the
      // single transport failure must surface with the localized wording.
      stream: async () => { throw new Error('Chat stream failed with HTTP 404'); },
      suggestions: { get: async () => ({ id: 'suggestions-1', questions: [] }), recordEvent: async () => undefined },
      approvals: { resolveTool: async () => undefined, cancelOAuth: async () => undefined, resolveOAuth: async () => undefined },
      artifacts: { message: async () => [], download: async () => ({ body: new Blob(), contentType: 'application/octet-stream' }) },
      steer: { enqueue: async () => undefined },
      stop: async () => undefined,
    },
  };
  (globalThis.window as { history: { pushState: (...args: unknown[]) => void } }).history.pushState = () => undefined;
  const props = await renderChatRoutePage({
    client,
    scopeController,
    location: 'http://weknora.test/platform/creatChat',
  });

  await assert.rejects(
    () => props.send({ content: 'hello', status: 'pending' }),
    /流式连接失败: HTTP 404/,
  );
});
