import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

import { assistantTimelineItems, isBookmarkActionAvailable, isFeedbackAvailable, MessageList, writeClipboardText } from './message-list.tsx';
import type { ChatMessage } from '@weknora/contracts';
import React from 'react';
import { act } from 'react';
import { resolveChatCopy } from './chat-copy.ts';
import type { Root } from 'react-dom/client';
Object.assign(globalThis, { React });
const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');

type ResolveFn = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveReactDom: ResolveFn = (specifier, context, nextResolve) => {
  if (specifier === 'react-dom/client') return { shortCircuit: true, url: new URL('../../../../apps/web/node_modules/react-dom/client.js', import.meta.url).href };
  return nextResolve(specifier, context);
};
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveFn }) => void };
hooks.registerHooks?.({ resolve: resolveReactDom });
const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/chat' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');

// Rendering assertions for MessageList (separators, timestamps, copy button,
// typing indicator, scroll-to-bottom) live in the web chat integration suite;
// this package test keeps the host-owned bookmark contract local to the view.

test('writeClipboardText prefers the async clipboard API', async () => {
  const seen: string[] = [];
  await writeClipboardText('copied text', { writeText: async (t) => { seen.push(t); } });
  assert.deepEqual(seen, ['copied text']);
});

test('assistant bookmark action follows the host-owned Vue manual-editor capability', () => {
  assert.equal(isBookmarkActionAvailable(), false);
  assert.equal(isBookmarkActionAvailable(() => undefined), true);
});

test('feedback buttons only render when host provides onRateMessage', () => {
  assert.equal(isFeedbackAvailable(), false);
  assert.equal(isFeedbackAvailable(() => undefined), true);
});

/*
 * Vue main-face contract (R464): a plain chat turn's reasoning stream is never
 * rendered as a fold block. frontend/src/views/chat/components/botmsg.vue only
 * renders thinking through deepThink (`<think>` tags parsed out of content)
 * or the agent-mode AgentStreamDisplay timeline; the `thinking` field and the
 * persisted `agent_steps[].reasoning_content/thought` text that
 * assistantMessageExtras() collects are NOT displayed on the Vue main face,
 * so the simplified React timeline drops the reasoning item too.
 */
test('assistant timeline keeps Vue tool order and finish node without a reasoning item', () => {
  const message = {
    id: 'assistant-1',
    session_id: 'session-1',
    role: 'assistant',
    content: 'done',
    is_completed: true,
    thinking: 'plan',
    tool_calls: [
      { id: 'tool-1', name: 'search_docs', status: 'completed' },
      { id: 'tool-2', name: 'write_file', status: 'failed' },
    ],
  } as ChatMessage;

  assert.deepEqual(assistantTimelineItems(message), [
    { kind: 'tool', id: 'tool-1', name: 'search_docs', status: 'completed' },
    { kind: 'tool', id: 'tool-2', name: 'write_file', status: 'failed' },
    { kind: 'finish' },
  ]);
  assert.deepEqual(assistantTimelineItems({ ...message, is_completed: false }), [
    { kind: 'tool', id: 'tool-1', name: 'search_docs', status: 'completed' },
    { kind: 'tool', id: 'tool-2', name: 'write_file', status: 'failed' },
  ]);
});

test('reasoning-only assistant data renders no timeline at all like the Vue main face', () => {
  const message = {
    id: 'assistant-1',
    session_id: 'session-1',
    role: 'assistant',
    content: 'done',
    is_completed: true,
    thinking: 'plan',
    agent_steps: [{ iteration: 0, reasoning_content: 'plan the search', thought: 'more reasoning' }],
  } as unknown as ChatMessage;
  assert.deepEqual(assistantTimelineItems(message), []);
});

test('underfilled history still exposes an accessible load-older action', () => {
  const markup = renderToStaticMarkup(React.createElement(MessageList, {
    copy: resolveChatCopy('en-US'),
    messages: [{ id: 'history-1', session_id: 's1', role: 'user', content: 'first history page', is_completed: true } as ChatMessage],
    hasMore: true, loadingOlder: false, onLoadOlder() {},
  }));
  assert.match(markup, /first history page/);
  assert.match(markup, /<button[^>]*aria-label="Load more"/);
});

test('load-older action reflects loading state and is unavailable without more history', () => {
  const loading = renderToStaticMarkup(React.createElement(MessageList, {
    copy: resolveChatCopy('en-US'),
    messages: [{ id: 'history-1', session_id: 's1', role: 'user', content: 'first history page', is_completed: true } as ChatMessage],
    hasMore: true, loadingOlder: true, onLoadOlder() {},
  }));
  assert.match(loading, /Loading/);
  assert.match(loading, /disabled=""/);
  const exhausted = renderToStaticMarkup(React.createElement(MessageList, {
    copy: resolveChatCopy('en-US'),
    messages: [{ id: 'history-1', session_id: 's1', role: 'user', content: 'first history page', is_completed: true } as ChatMessage],
    hasMore: false, onLoadOlder() {},
  }));
  assert.match(exhausted, /first history page/);
  assert.doesNotMatch(exhausted, /Load more/);
});

test('activating Load more invokes history loading once and loading state blocks activation', async () => {
  const container = document.createElement('div');
  const root: Root = createRoot(container);
  const messages = [{ id: 'history-1', session_id: 's1', role: 'user', content: 'first history page', is_completed: true } as ChatMessage];
  let calls = 0;
  const props = { copy: resolveChatCopy('en-US'), messages, hasMore: true, loadingOlder: false, onLoadOlder: () => { calls += 1; } };
  try {
    await act(async () => { root.render(React.createElement(MessageList, props)); });
    const loadButton = container.querySelector<HTMLButtonElement>('button[aria-label="Load more"]');
    assert.ok(loadButton);
    await act(async () => { loadButton.click(); });
    assert.equal(calls, 1);

    await act(async () => { root.render(React.createElement(MessageList, { ...props, loadingOlder: true })); });
    const loadingButton = container.querySelector<HTMLButtonElement>('button[aria-label="Loading..."]');
    assert.ok(loadingButton);
    await act(async () => { loadingButton.click(); });
    assert.equal(calls, 1);
  } finally {
    await act(async () => { root.unmount(); });
    container.remove();
  }
});

test('a generating suggestion set renders the follow-up loading hint, not the grid', async () => {
  const container = document.createElement('div');
  const root: Root = createRoot(container);
  try {
    await act(async () => {
      root.render(React.createElement(MessageList, {
        copy: resolveChatCopy('en-US'),
        messages: [],
        suggestions: { id: 'sg-1', session_id: 's1', assistant_message_id: 'm1', status: 'generating', allow_regenerate: false, questions: [] },
      }));
    });
    assert.ok(container.querySelector('.wk-chat-suggestions-loading'), 'loading hint renders');
    assert.ok((container.textContent ?? '').includes('Loading suggested questions'));
    assert.equal(container.querySelectorAll('.wk-chat-suggestions-grid').length, 0, 'no question grid while generating');
  } finally {
    await act(async () => { root.unmount(); });
    container.remove();
  }
});
