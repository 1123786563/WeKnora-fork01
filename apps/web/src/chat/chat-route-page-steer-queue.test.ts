import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
import { build } from 'esbuild';

interface CapturedChatPageProps {
  send(submission: { content: string; status: 'pending' }): Promise<void>;
  onSteer?(content: string, mentionedItems?: readonly unknown[]): Promise<void>;
  onSteerPromote?(steerId: string): void | Promise<void>;
  onSteerRemove?(steerId: string): void | Promise<void>;
  steerQueue?: readonly unknown[];
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
    filename: 'chat-route-page-steer-queue-test-bundle.cjs',
  });
  return module.exports.captureChatRoutePageProps({
    client: input.client,
    scopeController: input.scopeController,
  });
}

function baseClient(steer: Record<string, unknown>, stream: unknown): Record<string, unknown> {
  return {
    sessions: {
      create: async ({ title }: { title: string }) => ({ id: 'session-steer', title, is_pinned: false }),
      messages: async () => [
        { id: 'user-1', session_id: 'session-steer', role: 'user', content: 'hello', is_completed: true },
        { id: 'assistant-1', session_id: 'session-steer', role: 'assistant', content: 'hi', is_completed: true },
      ],
    },
    configuration: {
      agents: {
        listWithState: async () => ({
          items: [{ id: 'agent-1', name: 'Agent One', config: { model_id: 'm1' } }],
          disabledOwnAgentIds: [],
        }),
      },
      mcp: { oauth: {} },
    },
    chat: {
      stream,
      suggestions: { get: async () => ({ id: 'suggestions-1', questions: [] }), recordEvent: async () => undefined },
      approvals: {
        resolveTool: async () => undefined,
        cancelOAuth: async () => undefined,
        resolveOAuth: async () => undefined,
      },
      artifacts: {
        message: async () => [],
        download: async () => ({ body: new Blob(), contentType: 'application/octet-stream' }),
      },
      steer,
      stop: async () => undefined,
    },
  };
}

const scopeController = {
  current: () => ({ scope: { tenantId: 'tenant-1' }, signal: undefined }),
  isCurrent: () => true,
};

/*
 * R473-A2 — the queued follow-up becomes a chip the host tracks locally (Vue
 * chat/index.vue steerQueue). While the agent turn streams, a steer enqueue
 * POSTs /steer and the queue item settles onto the server-issued steer id;
 * the chip actions map to the Vue endpoints: promote → /steer/:id/inject,
 * remove → DELETE /steer/:id.
 */
test('a steer during a running agent turn enqueues server-side and wires the chip action handlers', async () => {
  const calls: string[] = [];
  // The stream stays open: the first answer event flips the phase to streaming
  // so buildSteerAction routes the follow-up through the enqueue branch.
  const stream = async (_options: unknown, feed: (event: unknown) => void) => {
    feed({ type: 'answer', content: 'working' });
    await new Promise(() => undefined);
  };
  const client = baseClient({
    enqueue: async (sessionId: string, input: { steerId?: string }) => {
      calls.push(`enqueue:${sessionId}:${input.steerId}`);
      return { success: true as const, status: 'queued' as const, steer_id: 'steer-server-1', assistant_message_id: 'assistant-1', delivery: 'after' as const };
    },
    promote: async (sessionId: string, steerId: string) => {
      calls.push(`promote:${sessionId}:${steerId}`);
      return { success: true as const, status: 'queued' as const, steer_id: steerId, assistant_message_id: 'assistant-1', delivery: 'inject' as const };
    },
    remove: async (sessionId: string, steerId: string) => {
      calls.push(`remove:${sessionId}:${steerId}`);
      return { success: true as const, status: 'deleted' as const, steer_id: steerId, removed: true };
    },
    list: async () => ({ success: true as const, items: [] }),
  }, stream);

  const props = await renderChatRoutePage({ client, scopeController, location: 'http://weknora.test/platform/creatChat' });
  (globalThis.window as { history: { pushState: (...args: unknown[]) => void } }).history.pushState = () => undefined;
  const streaming = props.send({ content: 'hello', status: 'pending' });
  await new Promise((resolve) => setImmediate(resolve));
  await props.onSteer!('补充一下');
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(calls.filter((call) => call.startsWith('enqueue:')).length, 1, 'follow-up while streaming POSTs /steer once');
  assert.match(calls[0]!, /enqueue:session-steer:steer-/);
  // The SSR snapshot is captured before the queued chip state lands (no
  // re-render in static markup); cross-realm arrays need a length check.
  assert.equal((props.steerQueue ?? []).length, 0, 'initial SSR snapshot carries the empty queue');

  await props.onSteerPromote!('steer-server-1');
  await new Promise((resolve) => setImmediate(resolve));
  assert.ok(calls.includes('promote:session-steer:steer-server-1'), 'chip promote hits /steer/:id/inject');

  await props.onSteerRemove!('steer-server-1');
  await new Promise((resolve) => setImmediate(resolve));
  assert.ok(calls.includes('remove:session-steer:steer-server-1'), 'chip remove hits DELETE /steer/:id');

  void streaming;
});

/*
 * R474-A2 N1 — Vue handleSteerMsg new_run race (chat/index.vue ~899): the
 * steer POST started while the turn streamed, but by the time it answers
 * new_run the page is idle (no run to join, nothing queued server-side).
 * Vue drops the queue item and falls back to sendMsg — the message must
 * reach the send path, never be silently dropped.
 */
test('an enqueue resolving new_run after the turn settled degrades to a plain send', async () => {
  const streamCalls: Array<{ body?: { query?: string } }> = [];
  let firstFeed: ((event: unknown) => void) | undefined;
  let releaseFirstStream: (() => void) | undefined;
  const stream = async (options: { body?: { query?: string } }, feed: (event: unknown) => void) => {
    streamCalls.push(options);
    if (streamCalls.length === 1) {
      firstFeed = feed;
      feed({ type: 'answer', content: 'working' });
      await new Promise<void>((resolve) => { releaseFirstStream = resolve; });
    }
  };
  let resolveEnqueue: ((value: unknown) => void) | undefined;
  const client = baseClient({
    enqueue: () => new Promise((resolve) => { resolveEnqueue = resolve; }),
    list: async () => ({ success: true as const, items: [] }),
  }, stream);

  const props = await renderChatRoutePage({ client, scopeController, location: 'http://weknora.test/platform/creatChat' });
  (globalThis.window as { history: { pushState: (...args: unknown[]) => void } }).history.pushState = () => undefined;
  void props.send({ content: 'hello', status: 'pending' });
  for (let i = 0; i < 3; i += 1) await new Promise((resolve) => setImmediate(resolve));

  // Steer dispatched while streaming: the enqueue POST is in flight.
  const steerPromise = props.onSteer!('补充一下');
  for (let i = 0; i < 3; i += 1) await new Promise((resolve) => setImmediate(resolve));
  assert.equal(streamCalls.length, 1, 'the enqueue is still in flight: no send has happened yet');
  assert.ok(resolveEnqueue, 'the steer enqueue POST is pending');

  // The turn settles before the enqueue POST answers.
  firstFeed!({ type: 'complete' });
  releaseFirstStream!();
  for (let i = 0; i < 5; i += 1) await new Promise((resolve) => setImmediate(resolve));

  resolveEnqueue!({ success: true, status: 'new_run', steer_id: 'steer-race-1' });
  await steerPromise;
  for (let i = 0; i < 5; i += 1) await new Promise((resolve) => setImmediate(resolve));

  assert.ok(streamCalls.length >= 2, 'the idle new_run degrades to a plain send instead of dropping the message');
  assert.equal(streamCalls.at(-1)?.body?.query, '补充一下', 'the degraded send carries the steered content');
});

test('a steer enqueue the server already injected leaves no queue residue', async () => {
  const calls: string[] = [];
  const stream = async (_options: unknown, feed: (event: unknown) => void) => {
    feed({ type: 'answer', content: 'working' });
    await new Promise(() => undefined);
  };
  const client = baseClient({
    enqueue: async () => ({ success: true as const, status: 'already_injected' as const, steer_id: 'steer-x' }),
    list: async () => ({ success: true as const, items: [] }),
  }, stream);

  const props = await renderChatRoutePage({ client, scopeController, location: 'http://weknora.test/platform/creatChat' });
  (globalThis.window as { history: { pushState: (...args: unknown[]) => void } }).history.pushState = () => undefined;
  const streaming = props.send({ content: 'hello', status: 'pending' });
  await new Promise((resolve) => setImmediate(resolve));
  await props.onSteer!('补充一下');
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(calls.length, 0);
  void streaming;
});

/*
 * Vue contract wiring, asserted on the host source (same pattern as
 * chat-page.test.ts): stop confirms clear the whole queue
 * (handleStopConfirmed), the injected SSE user message drops its queue entry
 * (onUserMessageInjected), and a completed turn re-syncs the queue with the
 * server before the follow-up attach (flushSteerAfterTurn → hydrate).
 */
test('ChatRoutePage wires the Vue steer queue lifecycle into the host stream pipeline', () => {
  const source = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  assert.match(source, /applySteerQueue\(clearSteerQueue\)/, 'stop confirmed clears the local steer queue');
  assert.match(source, /steerQueueChips\(steerQueue\)/, 'queue state projects into the composer chips');
  assert.match(source, /client\.chat\.steer\.promote\(/, 'promote action calls the inject endpoint');
  assert.match(source, /client\.chat\.steer\.remove\(/, 'remove action calls the delete endpoint');
  assert.match(source, /client\.chat\.steer\.list\(/, 'completed turns re-sync the queue with the server');
  assert.match(source, /onSteerPromote=\{/, 'ChatPage receives the promote handler');
  assert.match(source, /onSteerRemove=\{/, 'ChatPage receives the remove handler');
});

/*
 * R474-A2 — the streaming steer composer carries the Vue inject shortcut
 * (Input-field.vue onKeydown → injectCurrentInput): plain Enter queues
 * ('after'), ⌘Enter/Alt+Enter either injects the typed draft (delivery
 * 'inject' flows through onSteer → buildSteerAction) or — with an empty
 * draft — promotes the first queued chip.
 */
test('the streaming steer composer and the host steer path carry the inject shortcut contract', async () => {
  const steerSubmitSource = readFileSync(new URL('./steer-submit.ts', import.meta.url), 'utf8');
  assert.match(steerSubmitSource, /delivery: SteerDeliveryMode/, 'the steer enqueue input models both delivery modes');
  const hostSource = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  assert.match(hostSource, /retrySteerId\?: string, delivery: 'after' \| 'inject' = 'after'/, 'the host steer handler accepts the delivery mode');
  assert.match(hostSource, /delivery: retrySteerId \? undefined : delivery/, 'the dispatch forwards the shortcut delivery to the enqueue input');

  const pageSource = readFileSync(new URL('../../../../packages/views/src/chat/page.tsx', import.meta.url), 'utf8');
  assert.match(pageSource, /onKeyDown=\{handleDraftKeyDown\}[^>]*disabled=\{busy\}/, 'the steer textarea handles keydown (Enter queue / ⌘Enter inject)');
  assert.match(pageSource, /isSteerInjectShortcut\(event, true\)/, 'the inject shortcut gates the promote-or-inject branch');
  assert.match(pageSource, /resolveSteerInjectAction\(\{ draft, steerQueue \}\)/, 'an empty draft resolves onto the first queued chip');
  assert.match(pageSource, /steerQueue=\{props\.steerQueue\} onSteerPromote=\{props\.onSteerPromote\}/, 'the page forwards the queue and promote handler to the steer composer');
});
