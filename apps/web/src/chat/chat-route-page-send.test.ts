import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';
import { build } from 'esbuild';

interface CapturedChatPageProps {
  send(submission: { content: string; status: 'pending' }): Promise<void>;
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
              return { deleteConfirmBody: 'Delete this conversation? This cannot be undone.', knowledgeBasesLoadFailed: 'Failed to load knowledge bases' };
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
