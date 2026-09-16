import assert from 'node:assert/strict';
import test from 'node:test';

import { initialChatStreamState, reduceChatStream } from './reducer.ts';

test('accumulates answer/thinking and handles tool approval lifecycle', () => {
  let state = initialChatStreamState();
  state = reduceChatStream(state, { response_type: 'thinking', event_id: '1', content: 'plan' });
  state = reduceChatStream(state, { response_type: 'answer', event_id: '2', content: 'hello' });
  state = reduceChatStream(state, { response_type: 'tool_call', event_id: '3', data: { tool_call_id: 'tool-1', tool_name: 'search' } });
  state = reduceChatStream(state, { response_type: 'tool_approval_required', event_id: '4', data: { pending_id: 'approval-1', tool_call_id: 'tool-1' } });
  state = reduceChatStream(state, { response_type: 'tool_approval_resolved', event_id: '5', data: { pending_id: 'approval-1', decision: 'approved' } });
  assert.equal(state.thinking, 'plan');
  assert.equal(state.answer, 'hello');
  assert.equal(state.approvals['approval-1']?.status, 'resolved');
  assert.equal(state.toolCalls['tool-1']?.name, 'search');
});

test('deduplicates event ids and preserves terminal state', () => {
  let state = initialChatStreamState();
  state = reduceChatStream(state, { response_type: 'answer', event_id: 'same', content: 'a' });
  state = reduceChatStream(state, { response_type: 'answer', event_id: 'same', content: 'duplicate' });
  state = reduceChatStream(state, { response_type: 'complete', event_id: 'done' });
  assert.equal(state.answer, 'a');
  assert.equal(state.phase, 'completed');
  assert.equal(state.lastEventId, 'done');
});

test('unknown response types do not fabricate completion', () => {
  const state = reduceChatStream(initialChatStreamState(), { response_type: 'future_event' });
  assert.equal(state.phase, 'idle');
});

test('retains the persisted assistant message id needed for history actions', () => {
  const state = reduceChatStream(initialChatStreamState(), {
    response_type: 'answer',
    message_id: 'assistant-1',
    content: 'answer',
  });
  assert.equal(state.assistantMessageId, 'assistant-1');
});

test('tracks MCP OAuth approval lifecycle separately from tool approval', () => {
  let state = initialChatStreamState();
  state = reduceChatStream(state, {
    response_type: 'mcp_oauth_required',
    event_id: 'oauth-1',
    data: { pending_id: 'pending-oauth', service_id: 'svc-1', service_name: 'Docs', mcp_tool_name: 'search_docs' },
  });
  assert.equal(state.oauthApprovals['pending-oauth']?.status, 'pending');
  assert.equal(state.oauthApprovals['pending-oauth']?.serviceId, 'svc-1');
  state = reduceChatStream(state, {
    response_type: 'mcp_oauth_resolved',
    event_id: 'oauth-2',
    data: { pending_id: 'pending-oauth', service_id: 'svc-1', authorized: true },
  });
  assert.equal(state.oauthApprovals['pending-oauth']?.status, 'resolved');
  assert.equal(state.oauthApprovals['pending-oauth']?.authorized, true);
});

test('session_title patches the session title on the stream state', () => {
  let state = reduceChatStream(initialChatStreamState(), {
    response_type: 'session_title',
    event_id: 'title-1',
    content: 'New title',
    data: { session_id: 'session-1', title: 'New title' },
  });
  assert.equal(state.sessionTitle, 'New title');
  // data.title wins over bare content when both exist
  state = reduceChatStream(state, {
    response_type: 'session_title',
    event_id: 'title-2',
    content: 'from content',
    data: { session_id: 'session-1', title: 'from data' },
  });
  assert.equal(state.sessionTitle, 'from data');
});

test('artifacts_pending toggles during streaming and clears on completion', () => {
  let state = initialChatStreamState();
  state = reduceChatStream(state, { response_type: 'answer', event_id: 'a', content: 'x' });
  assert.equal(state.artifactsPending, false);
  state = reduceChatStream(state, { response_type: 'artifacts_pending', event_id: 'b', data: { count: 2 } });
  assert.equal(state.artifactsPending, true);
  state = reduceChatStream(state, { response_type: 'complete', event_id: 'c' });
  assert.equal(state.artifactsPending, false);
});

test('user_message_injected records an injected user bubble', () => {
  let state = reduceChatStream(initialChatStreamState(), {
    response_type: 'user_message_injected',
    event_id: 'inject-1',
    data: { steer_id: 'steer-1', message_id: 'assistant-1', content: 'hello again', user_message_id: 'user-1' },
  });
  assert.deepEqual(state.injectedUserMessages, [{
    steerId: 'steer-1',
    content: 'hello again',
    userMessageId: 'user-1',
  }]);
  state = reduceChatStream(state, {
    response_type: 'user_message_injected',
    event_id: 'inject-2',
    data: { steer_id: 'steer-2', content: 'and more' },
  });
  assert.equal(state.injectedUserMessages.length, 2);
  assert.equal(state.injectedUserMessages[1]?.userMessageId, undefined);
});
