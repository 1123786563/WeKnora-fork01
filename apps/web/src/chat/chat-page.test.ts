import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import React from 'react';

import { ChatPage } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

test('chat page exposes the selected agent and server-disabled state at the chat entry', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    agents: [{ id: 'agent-1', name: 'Research' }, { id: 'agent-2', name: 'Disabled', disabled: true }],
    selectedAgentId: 'agent-1',
    onAgentChange: () => undefined,
  }));

  assert.match(html, /id="wk-chat-agent"/);
  assert.match(html, /value="agent-1"/);
  assert.match(html, /Research/);
  assert.match(html, /Disabled · disabled/);
});
