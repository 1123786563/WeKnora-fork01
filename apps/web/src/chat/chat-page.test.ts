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
