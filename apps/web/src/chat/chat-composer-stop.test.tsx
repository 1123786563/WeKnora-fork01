import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
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
