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
 * stay out of the rendered HTML.
 *
 * 2026-09-19 live-round contract (Vue AgentStreamDisplay): once a turn is
 * completed the tool timeline folds into the single 检索完成 summary with the
 * cited-document count (references open from it); the raw tool timeline only
 * renders while the turn is still streaming.
 */
test('completed assistant messages fold tool calls into the retrieval-done summary', () => {
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
        references: [{ title: 'Doc A', content: 'chunk', chunk_ids: ['c1'] }],
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
  // Completed face: collapsed 检索完成 + 引用了{count}篇文档, no raw tool row.
  assert.match(html, /检索完成/);
  assert.match(html, /引用了1篇文档/);
  assert.doesNotMatch(html, /search_docs/);
});

test('streaming assistant messages keep the tool timeline visible', () => {
  const html = renderToStaticMarkup(React.createElement(MessageList, {
    messages: [
      {
        id: 'assistant-live',
        session_id: 'session-1',
        role: 'assistant',
        content: '',
        agent_steps: [
          { iteration: 0, thought: 'plan the search', tool_calls: [{ id: 'call-1', name: 'search_docs', args: {} }] },
        ],
      },
    ],
  }));
  assert.match(html, /search_docs/);
  assert.doesNotMatch(html, /plan the search/);
});
