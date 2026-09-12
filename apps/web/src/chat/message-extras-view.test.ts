import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import React from 'react';

import { MessageList } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

test('assistant messages render persisted thinking and tool calls as view data', () => {
  const html = renderToStaticMarkup(React.createElement(MessageList, {
    messages: [{
      id: 'assistant-1',
      session_id: 'session-1',
      role: 'assistant',
      content: 'final answer',
      is_completed: true,
      agent_steps: [
        { iteration: 0, thought: 'plan the search', tool_calls: [{ id: 'call-1', name: 'search_docs', args: {} }] },
      ],
    }],
  }));
  assert.match(html, /final answer/);
  assert.match(html, /plan the search/);
  assert.match(html, /search_docs/);
});

test('plain assistant and user messages render without an extras section', () => {
  const html = renderToStaticMarkup(React.createElement(MessageList, {
    messages: [{ id: 'u1', session_id: 's', role: 'user', content: 'hello' }],
  }));
  assert.doesNotMatch(html, /Thinking/);
});
