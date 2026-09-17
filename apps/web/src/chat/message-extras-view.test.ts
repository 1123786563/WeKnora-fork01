import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import React from 'react';

import { MessageList } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

/*
 * R464 Vue-parity contract: botmsg.vue never renders the reasoning trace
 * (`thinking` / `agent_steps[].thought` / `reasoning_content`) on the main
 * chat face — only `<think>` tags inside content reach the deepThink fold and
 * the agent timeline is a separate surface. The persisted reasoning text must
 * stay out of the rendered HTML while tool calls remain visible.
 */
test('assistant messages render tool calls but keep the reasoning trace unrendered', () => {
  const html = renderToStaticMarkup(React.createElement(MessageList, {
    messages: [
      {
        id: 'assistant-1',
        session_id: 'session-1',
        role: 'assistant',
        content: 'final answer',
        is_completed: true,
        agent_steps: [
          { iteration: 0, thought: 'plan the search', tool_calls: [{ id: 'call-1', name: 'search_docs', args: {} }] },
        ],
      },
      {
        id: 'assistant-2',
        session_id: 'session-1',
        role: 'assistant',
        content: 'live answer',
        is_completed: true,
        thinking: 'live reasoning trace',
      },
    ],
  }));
  assert.match(html, /final answer/);
  assert.doesNotMatch(html, /plan the search/);
  assert.doesNotMatch(html, /live reasoning trace/);
  assert.match(html, /search_docs/);
});

test('plain assistant and user messages render without an extras section', () => {
  const html = renderToStaticMarkup(React.createElement(MessageList, {
    messages: [{ id: 'u1', session_id: 's', role: 'user', content: 'hello' }],
  }));
  assert.doesNotMatch(html, /Thinking/);
});
