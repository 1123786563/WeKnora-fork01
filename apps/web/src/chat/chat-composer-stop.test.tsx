import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// B1 question-minimap（ChatPage 渲染路径）经 window.requestAnimationFrame
// 调度测量；jsdom 无 raf，window 侧与 globalThis 侧都垫 setTimeout 帧垫片
// （同 settings 域判例 SkillSettingsPanel.test）。
const w = dom.window as unknown as { requestAnimationFrame?: unknown; cancelAnimationFrame?: unknown; matchMedia?: unknown };
w.requestAnimationFrame = w.requestAnimationFrame ?? ((cb: (t: number) => void) => setTimeout(() => cb(Date.now()), 16));
w.cancelAnimationFrame = w.cancelAnimationFrame ?? ((id: ReturnType<typeof setTimeout>) => clearTimeout(id));
(globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame
  = (globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame ?? w.requestAnimationFrame;
(globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame
  = (globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame ?? w.cancelAnimationFrame;
// B1 question-minimap 指针粗细探测（question-minimap.tsx setIsCoarsePointer）：
// jsdom 无 matchMedia，垫 coarse=false 的最小桩。
w.matchMedia = w.matchMedia ?? ((query: string) => ({ matches: false, media: query, addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {}, onchange: null, dispatchEvent: () => false }));
const { createRoot } = await import('react-dom/client');
const { ChatComposer, ChatPage, resolveChatCopy } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

/*
 * Vue contract (Input-field.vue control-right, lines ~2844-2853):
 * `isReplying && (!canSteer || !query.trim())` swaps the circular send button
 * for a stop button (aria-label input.stopGeneration); clicking it emits
 * stop-generation so the parent aborts the stream. `isReplying` flips true the
 * moment a turn is dispatched (sendMsg sets it before the fetch), NOT when the
 * first SSE event arrives.
 */

test('quick-answer streaming swaps the send button for a stop button that invokes onStop', async () => {
  const stops: number[] = [];
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatComposer copy={resolveChatCopy('zh-CN')} draft="仍在输入" onDraftChange={() => undefined} onSubmit={() => undefined} streaming canSteer={false} onStop={() => stops.push(1)} />));
  const stopButton = container.querySelector<HTMLButtonElement>('button[aria-label="停止生成"]');
  assert.ok(stopButton, 'streaming without steer support must render the stop button');
  assert.equal(container.querySelector('button[data-guide="chat-send"]'), null, 'the send button is replaced, not merely disabled');
  await act(async () => stopButton?.click());
  assert.deepEqual(stops, [1]);
});

test('agent streaming with a typed draft keeps the send button for steering (Vue steerAfter branch)', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatComposer copy={resolveChatCopy('zh-CN')} draft="排队追问" onDraftChange={() => undefined} onSubmit={() => undefined} streaming canSteer onStop={() => undefined} />));
  assert.ok(container.querySelector<HTMLButtonElement>('button[data-guide="chat-send"]'), 'canSteer + draft keeps send available for queueing');
  assert.equal(container.querySelector('button[aria-label="停止生成"]'), null);
});

/*
 * R470-A2 / R469 N019: React used to keep a disabled send button during the
 * pre-stream window (request dispatched, first SSE event not yet arrived) while
 * Vue already showed stop — Vue's isReplying covers the whole turn, whereas
 * React's `streaming` prop only reflected `phase === 'streaming'`. The composer
 * must receive the turn-running signal (sending || streaming).
 */
/*
 * R471-A1 truth table (Vue Input-field.vue): the composer state a user can
 * actually observe while a turn is running is the cross product of
 * canSteer (isAgentStreamSession — agent pipeline vs quick-answer) x query
 * (draft) x isReplying. Vue clears the query the moment a message is emitted
 * (createSession -> clearvalue on BOTH the send and the steer path), so the
 * default post-send state is "empty draft" and the stop button shows; typing
 * text into an agent-pipeline session swaps stop back to a send button whose
 * label is the steer copy (input.steerAfter), NOT input.send.
 *
 * streaming | canSteer | draft   | control-right
 * ----------+----------+---------+-------------------------------
 * true      | false    | any     | stop (quick-answer: only action)
 * true      | true     | empty   | stop (draft was cleared on send)
 * true      | true     | text    | send labelled steerAfter (queue)
 * false     | any      | any     | send labelled input.send
 */
const CONTROL_RIGHT_TRUTH_TABLE = [
  { name: 'quick-answer + leftover draft (host did not clear) still shows stop', streaming: true, canSteer: false, draft: '仍在输入', expectStop: true },
  { name: 'quick-answer + empty draft shows stop', streaming: true, canSteer: false, draft: '', expectStop: true },
  { name: 'agent pipeline + draft cleared on send shows stop (Vue clearvalue)', streaming: true, canSteer: true, draft: '', expectStop: true },
  { name: 'agent pipeline + typed draft keeps send for queueing', streaming: true, canSteer: true, draft: '排队追问', expectStop: false },
  { name: 'idle quick-answer shows the normal send button', streaming: false, canSteer: false, draft: '你好', expectStop: false },
  { name: 'idle agent pipeline shows the normal send button', streaming: false, canSteer: true, draft: '你好', expectStop: false },
] as const;
for (const row of CONTROL_RIGHT_TRUTH_TABLE) {
  test(`composer control-right truth table: ${row.name}`, async () => {
    const { streaming, canSteer, draft, expectStop } = row;
    const container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    await act(async () => root?.render(<ChatComposer copy={resolveChatCopy('zh-CN')} draft={draft} onDraftChange={() => undefined} onSubmit={() => undefined} streaming={streaming} canSteer={canSteer} onStop={() => undefined} />));
    const stopButton = container.querySelector<HTMLButtonElement>('button[aria-label="停止生成"]');
    const sendButton = container.querySelector<HTMLButtonElement>('button[data-guide="chat-send"]');
    if (expectStop) {
      assert.ok(stopButton, 'this combination must render the stop button');
      assert.equal(sendButton, null, 'the send button is replaced, not merely disabled');
    } else {
      assert.ok(sendButton, 'this combination must render the send button');
      assert.equal(stopButton, null);
    }
  });
}

/*
 * Vue steer-mode labelling (Input-field.vue ~2850-2853): while replying with
 * steer capability and text in the box, the tooltip and aria-label switch to
 * input.steerAfter ("完成后发送") — the button queues a follow-up, it does not
 * start a new turn. Idle keeps input.send.
 */
test('send button carries the steer copy while streaming with steer capability, and the plain send copy when idle', async () => {
  for (const [streaming, expectedLabel] of [[true, '完成后发送'], [false, '发送']] as const) {
    const container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    await act(async () => root?.render(<ChatComposer copy={resolveChatCopy('zh-CN')} draft="追问" onDraftChange={() => undefined} onSubmit={() => undefined} streaming={streaming} canSteer onStop={() => undefined} />));
    const sendButton = container.querySelector<HTMLButtonElement>('button[data-guide="chat-send"]');
    assert.equal(sendButton?.getAttribute('aria-label'), expectedLabel, `aria-label while streaming=${streaming}`);
    assert.equal(sendButton?.getAttribute('title'), `${expectedLabel} · Enter`, `title while streaming=${streaming}`);
  }
});

/*
 * Vue createSession clears the query right after emitting send-msg /
 * steer-msg (clearvalue, regardless of the later turn outcome). The React
 * composer is controlled: submitting must dispatch onDraftChange('') so the
 * host draft no longer pins the control-right in the send branch.
 */
test('submitting a draft clears it immediately (Vue clearvalue on the send path)', async () => {
  const submitted: string[] = [];
  const draftChanges: string[] = [];
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatComposer copy={resolveChatCopy('zh-CN')} draft="你好" onDraftChange={(value) => draftChanges.push(value)} onSubmit={(submission) => submitted.push(submission.content)} onStop={() => undefined} />));
  await act(async () => container.querySelector<HTMLButtonElement>('button[data-guide="chat-send"]')?.click());
  assert.deepEqual(submitted, ['你好']);
  assert.deepEqual(draftChanges, [''], 'the draft must be cleared the moment the message is dispatched');
});

/*
 * R471-A1 root cause 1: ChatPage derived canSteer from Boolean(onSteer), so
 * the web host (which always wires a steer fallback) never saw the quick-
 * answer "stop-only" state. An explicit canSteer=false must override the
 * presence of onSteer for BOTH the control-right swap and the separate steer
 * composer (Vue has no steer affordance on a quick-answer turn).
 */
test('ChatPage honors an explicit canSteer=false even when onSteer is wired', async () => {
  const steers: string[] = [];
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatPage
    sessions={[{ id: 'session-1', title: 'Chat', is_pinned: false }]}
    selectedSessionId="session-1"
    messages={[]}
    draft="未清空的输入"
    locale="zh-CN"
    onSelectSession={() => undefined}
    onCreateSession={() => undefined}
    onDraftChange={() => undefined}
    send={async () => undefined}
    onSteer={async (content) => { steers.push(content); }}
    canSteer={false}
    stream={{ phase: 'streaming', thinking: '', answer: '生成中', references: [], toolCalls: [], artifactsPending: false }}
    onStopStream={() => undefined}
  />));
  assert.ok(container.querySelector<HTMLButtonElement>('button[aria-label="停止生成"]'), 'explicit canSteer=false must force the stop button even with a non-empty draft');
  assert.equal(container.querySelector('button[data-guide="chat-send"]'), null);
  assert.equal(container.querySelector('#wk-chat-steer-draft'), null, 'a quick-answer turn must not render the steer composer');
});

test('ChatPage shows the stop button while a send is in flight before the first stream event', async () => {
  const stopped: number[] = [];
  let releaseSend: (() => void) | undefined;
  const send = () => new Promise<void>((resolve) => { releaseSend = resolve; });
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatPage
    sessions={[{ id: 'session-1', title: 'Chat', is_pinned: false }]}
    selectedSessionId="session-1"
    messages={[]}
    draft="hello"
    locale="zh-CN"
    onSelectSession={() => undefined}
    onCreateSession={() => undefined}
    onDraftChange={() => undefined}
    send={send}
    onStopStream={() => stopped.push(1)}
  />));
  // No onSteer → quick-answer composer → stop replaces send while replying.
  const sendButton = container.querySelector<HTMLButtonElement>('button[data-guide="chat-send"]');
  assert.ok(sendButton);
  assert.equal(sendButton?.disabled, false, 'draft is present so the send button is enabled before dispatch');
  await act(async () => sendButton?.click());
  const stopButton = container.querySelector<HTMLButtonElement>('button[aria-label="停止生成"]');
  assert.ok(stopButton, 'the in-flight send (stream not yet open) must already surface the stop button like Vue isReplying');
  await act(async () => stopButton?.click());
  assert.deepEqual(stopped, [1], 'clicking stop reaches the host stop pipeline');
  await act(async () => releaseSend?.());
});
