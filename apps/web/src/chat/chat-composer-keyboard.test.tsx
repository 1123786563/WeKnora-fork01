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
const { ChatComposer } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

test('submits on the Vue-compatible plain Enter shortcut', async () => {
  const submissions: string[] = [];
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatComposer draft="Ask Vue" onDraftChange={() => undefined} onSubmit={(submission) => submissions.push(submission.content)} />));
  const textarea = container.querySelector<HTMLTextAreaElement>('#wk-chat-draft');
  assert.ok(textarea);

  await act(async () => textarea?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })));

  assert.deepEqual(submissions, ['Ask Vue']);
});

/*
 * R474-A2 — Vue Input-field.vue injectCurrentInput: while replying on a
 * steer-capable turn, ⌘Enter (Alt+Enter off-Mac) injects the current draft;
 * with an empty draft it promotes the first queued steer chip instead
 * (firstQueuedSteer → emit('promote-steer')).
 */
test('⌘Enter promotes the first queued steer when the draft is empty while streaming', async () => {
  const submissions: string[] = [];
  const promoted: string[] = [];
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(
    <ChatComposer
      draft=""
      streaming
      canSteer
      steerQueue={[
        { steerId: 's-pending', content: 'posting', status: 'pending' },
        { steerId: 's-queued', content: 'queued follow-up', status: 'queued' },
      ]}
      onDraftChange={() => undefined}
      onSubmit={(submission) => submissions.push(submission.content)}
      onSteerPromote={(steerId) => promoted.push(steerId)}
    />,
  ));
  const textarea = container.querySelector<HTMLTextAreaElement>('#wk-chat-draft');
  assert.ok(textarea);

  await act(async () => textarea?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true, cancelable: true })));

  assert.deepEqual(promoted, ['s-queued'], 'the shortcut promotes the first queued chip');
  assert.deepEqual(submissions, [], 'an empty draft is not submitted');
});

test('⌘Enter with a typed draft submits it for immediate injection while streaming', async () => {
  const submissions: string[] = [];
  const promoted: string[] = [];
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(
    <ChatComposer
      draft="answer now please"
      streaming
      canSteer
      steerQueue={[{ steerId: 's-queued', content: 'queued follow-up', status: 'queued' }]}
      onDraftChange={() => undefined}
      onSubmit={(submission) => submissions.push(submission.content)}
      onSteerPromote={(steerId) => promoted.push(steerId)}
    />,
  ));
  const textarea = container.querySelector<HTMLTextAreaElement>('#wk-chat-draft');
  assert.ok(textarea);

  await act(async () => textarea?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true, cancelable: true })));

  assert.deepEqual(submissions, ['answer now please'], 'the draft wins over the queue');
  assert.deepEqual(promoted, []);
});
