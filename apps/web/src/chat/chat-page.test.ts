import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import React from 'react';

import { ChatPage } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

test('chat page exposes the selected agent and server-disabled state at the chat entry', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [{ id: 'session-1', title: 'Chat', is_pinned: true }],
    selectedSessionId: 'session-1',
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    agents: [{ id: 'agent-1', name: 'Research' }, { id: 'agent-2', name: 'Disabled', disabled: true }],
    selectedAgentId: 'agent-1',
    onAgentChange: () => undefined,
    toolApprovals: [{ pendingId: 'approval-1', toolName: 'search_docs', status: 'pending' }],
    oauthApprovals: [{ pendingId: 'oauth-1', serviceId: 'mcp-1', serviceName: 'Docs MCP', toolName: 'search', status: 'pending' }],
    onResolveToolApproval: async () => undefined,
    onAuthorizeOAuth: async () => undefined,
    onCancelOAuth: async () => undefined,
    onSteer: async () => undefined,
    onStopStream: () => undefined,
    onRenameSession: async () => undefined,
    onToggleSessionPin: async () => undefined,
    onDeleteSession: async () => undefined,
    terminal: { status: 'ready', output: '$ ls' },
    onOpenTerminal: async () => undefined,
    onTerminalInput: async () => undefined,
    onCloseTerminal: () => undefined,
    stream: {
      phase: 'streaming',
      thinking: 'checking sources',
      references: [{ title: 'Guide <safe>' }],
      toolCalls: [{ id: 'tool-1', name: 'search_docs', status: 'completed', result: { html: '<not markup>', api_key: 'secret' } }],
    },
  }));

  assert.match(html, /id="wk-chat-agent"/);
  assert.match(html, /value="agent-1"/);
  assert.match(html, /Research/);
  assert.match(html, /Disabled · disabled/);
  assert.match(html, /Approve search_docs/);
  assert.match(html, /Authorize Docs MCP/);
  assert.match(html, /Queue follow-up/);
  assert.match(html, /checking sources/);
  assert.match(html, /Guide &lt;safe&gt;/);
  assert.match(html, /search_docs/);
  assert.match(html, /&lt;not markup&gt;/);
  assert.match(html, /\[redacted\]/);
  assert.match(html, /Rename session-1/);
  assert.match(html, /Unpin session-1/);
  assert.match(html, /Delete session-1/);
  assert.match(html, /Open terminal/);
  assert.match(html, /\$ ls/);
  assert.match(html, /id="wk-chat-draft"[^>]*disabled=""/);
});

test('chat page hides the steer composer and stop button when idle', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [{ id: 'session-1', title: 'Chat', is_pinned: false }],
    selectedSessionId: 'session-1',
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    onSteer: async () => undefined,
    onStopStream: () => undefined,
    stream: { phase: 'idle', thinking: '', references: [], toolCalls: [] },
  }));
  assert.doesNotMatch(html, /Queue follow-up/);
  assert.doesNotMatch(html, /class="wk-chat-stop"/);
});

test('chat page shows the artifacts-pending indicator only while streaming', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [{ id: 'session-1', title: 'Chat', is_pinned: false }],
    selectedSessionId: 'session-1',
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    stream: { phase: 'streaming', thinking: '', references: [], toolCalls: [], artifactsPending: true },
  }));
  assert.match(html, /Artifacts pending/);
  assert.match(html, /Status: streaming/);
});
test('new-conversation view renders the agent picker and the agent suggested questions', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    agents: [{ id: 'agent-1', name: 'Research' }, { id: 'agent-2', name: 'Disabled agent', disabled: true }],
    selectedAgentId: 'agent-1',
    onAgentChange: () => undefined,
    starterQuestions: ['What is WeKnora?', 'How do I upload files?'],
    onStarterQuestionClick: () => undefined,
  }));
  // Agent picker (Vue AgentSelector semantics) renders the available agents.
  assert.match(html, /id="wk-chat-agent"/);
  assert.match(html, /value="agent-1"/);
  assert.match(html, /Research/);
  assert.match(html, /value="agent-2"/);
  assert.match(html, /Disabled agent · disabled/);
  // Empty-state starters from GET /api/v1/agents/:id/suggested-questions.
  assert.match(html, /wk-chat-starters/);
  assert.match(html, /Suggested questions/);
  assert.match(html, /What is WeKnora?/);
  assert.match(html, /How do I upload files?/);
});

test('new-conversation view renders no starters block without agent suggestions', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    starterQuestions: [],
  }));
  assert.doesNotMatch(html, /wk-chat-starters/);
});

test('starters are hidden once a session is open (message suggestions own that state)', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [{ id: 'session-1', title: 'Chat', is_pinned: false }],
    selectedSessionId: 'session-1',
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    starterQuestions: ['What is WeKnora?'],
  }));
  assert.doesNotMatch(html, /wk-chat-starters/);
});

test('new-conversation view renders the agent picker and the agent suggested questions', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    agents: [{ id: 'agent-1', name: 'Research' }, { id: 'agent-2', name: 'Disabled agent', disabled: true }],
    selectedAgentId: 'agent-1',
    onAgentChange: () => undefined,
    starterQuestions: ['What is WeKnora?', 'How do I upload files?'],
    onStarterQuestionClick: () => undefined,
  }));
  assert.match(html, /id="wk-chat-agent"/);
  assert.match(html, /value="agent-1"/);
  assert.match(html, /Research/);
  assert.match(html, /value="agent-2"/);
  assert.match(html, /Disabled agent · disabled/);
  assert.match(html, /wk-chat-starters/);
  assert.match(html, /Suggested questions/);
  assert.match(html, /What is WeKnora?/);
  assert.match(html, /How do I upload files?/);
});

test('new-conversation view renders no starters block without agent suggestions', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    starterQuestions: [],
  }));
  assert.doesNotMatch(html, /wk-chat-starters/);
});

test('starters are hidden once a session is open (message suggestions own that state)', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [{ id: 'session-1', title: 'Chat', is_pinned: false }],
    selectedSessionId: 'session-1',
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    starterQuestions: ['What is WeKnora?'],
  }));
  assert.doesNotMatch(html, /wk-chat-starters/);
});

test('new-conversation view renders the starter-questions skeleton while loading', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    agents: [{ id: 'agent-1', name: 'Research' }],
    selectedAgentId: 'agent-1',
    starterQuestions: [],
    starterQuestionsLoading: true,
  }));
  assert.match(html, /wk-chat-starters--loading/);
  assert.match(html, /aria-busy="true"/);
  assert.match(html, /wk-chat-starter-skeleton/);
});

test('chat page passes the typing indicator while streaming without assistant content', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [{ id: 'u1', session_id: 's', role: 'user', content: 'hello' }],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    stream: { phase: 'streaming', thinking: '', references: [], toolCalls: [] },
  }));
  assert.match(html, /wk-chat-typing/);
});

test('chat page hides the typing indicator once thinking or tool calls arrive', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [{ id: 'u1', session_id: 's', role: 'user', content: 'hello' }],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    stream: { phase: 'streaming', thinking: 'checking', references: [], toolCalls: [] },
  }));
  assert.doesNotMatch(html, /wk-chat-typing/);
});

test('message list renders separators, per-message timestamps, and the copy-answer button', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [
      { id: 'u1', session_id: 's', role: 'user', content: 'hello', created_at: new Date(2024, 2, 5, 9, 0, 0).toISOString() },
      { id: 'a1', session_id: 's', role: 'assistant', content: 'The answer is 42', created_at: new Date(2024, 2, 5, 9, 1, 0).toISOString() },
      { id: 'u2', session_id: 's', role: 'user', content: 'next day', created_at: new Date(2024, 2, 6, 9, 0, 0).toISOString() },
    ],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
  }));
  assert.match(html, /wk-chat-timestamp/);
  assert.match(html, /wk-chat-message-time/);
  assert.match(html, /wk-chat-copy/);
  assert.match(html, /aria-label="Copy answer"/);
  assert.match(html, /The answer is 42/);
  // Scroll-to-bottom only appears after the user scrolls up (client-only).
  assert.doesNotMatch(html, /wk-chat-scroll-bottom/);
});
