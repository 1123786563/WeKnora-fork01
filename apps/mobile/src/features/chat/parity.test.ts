import assert from 'node:assert/strict';
import test from 'node:test';
import { initialChatStreamState, reduceChatStream } from '@weknora/domain/chat/reducer';
import { buildMobileChatRequestBody, findRetryQuery, isCurrentChatRun, replayMobileChatEvents, selectAssistantMessageId, selectIncompleteAssistant, selectReferenceGroups, shouldRenderLiveAssistant, shouldRenderPendingUser } from './parity.ts';

test('mobile chat request carries the selected knowledge-base scope', () => {
  assert.deepEqual(buildMobileChatRequestBody('hello', ['kb-1'], ['attachment-1']), {
    query: 'hello',
    knowledge_base_ids: ['kb-1'],
    attachment_ids: ['attachment-1'],
    channel: 'mobile',
  });
});

test('mobile hides the live assistant after history has been reloaded', () => {
  assert.equal(shouldRenderLiveAssistant(false, 'Hello from Android'), false);
  assert.equal(shouldRenderLiveAssistant(true, 'Hello from Android'), true);
});

test('mobile replay uses the shared reducer for answer, approval, and references', () => {
  const events = [
    { response_type: 'answer', event_id: '1', content: 'hello' },
    { response_type: 'references', event_id: '2', data: { references: [{ knowledge_id: 'doc-1', knowledge_title: 'Guide', content: 'source' }] } },
    { response_type: 'tool_approval_required', event_id: '3', data: { pending_id: 'pending-1', tool_call_id: 'tool-1' } },
    { response_type: 'mcp_oauth_required', event_id: '4', data: { pending_id: 'oauth-1', service_id: 'service-1', service_name: 'Docs' } },
  ] as const;
  const state = replayMobileChatEvents(events);
  assert.equal(state.answer, 'hello');
  assert.equal(state.approvals['pending-1']?.status, 'pending');
  assert.equal(state.oauthApprovals['oauth-1']?.serviceName, 'Docs');
  assert.equal(selectReferenceGroups(state)[0]?.items[0]?.title, 'Guide');
  assert.equal(reduceChatStream(initialChatStreamState(), events[0]).answer, state.answer);
});

test('mobile resumes only an incomplete assistant message', () => {
  assert.equal(selectIncompleteAssistant([
    { id: 'u', session_id: 's', role: 'user', content: 'hi' },
    { id: 'done', session_id: 's', role: 'assistant', content: 'done', is_completed: true },
    { id: 'live', session_id: 's', role: 'assistant', content: 'partial', is_completed: false },
  ])?.id, 'live');
});

test('mobile finds the user query immediately preceding a failed assistant response', () => {
  assert.equal(findRetryQuery([
    { id: 'u-1', session_id: 's', role: 'user', content: 'first' },
    { id: 'a-1', session_id: 's', role: 'assistant', content: 'done', is_completed: true },
    { id: 'u-2', session_id: 's', role: 'user', content: 'retry me' },
    { id: 'a-2', session_id: 's', role: 'assistant', content: 'partial', is_completed: false },
  ], 'a-2'), 'retry me');
  assert.equal(findRetryQuery([
    { id: 'a-2', session_id: 's', role: 'assistant', content: 'partial', is_completed: false },
  ], 'a-2'), undefined);
});

test('mobile does not duplicate a user message after stream recovery reloads history', () => {
  const messages = [{ id: 'u', session_id: 's', role: 'user' as const, content: 'hello' }];
  assert.equal(shouldRenderPendingUser(messages, 'hello'), false);
  assert.equal(shouldRenderPendingUser(messages, 'new draft'), true);
});

test('mobile extracts the assistant id from the nested stream query event', () => {
  assert.equal(selectAssistantMessageId({ response_type: 'agent_query', data: { assistant_message_id: 'assistant-1' } }), 'assistant-1');
  assert.equal(selectAssistantMessageId({ message_id: 'assistant-2' }), 'assistant-2');
  assert.equal(selectAssistantMessageId({ response_type: 'error', data: { error: 'failed' } }), undefined);
});

test('mobile applies a stream event only to the session and run that created it', () => {
  assert.equal(isCurrentChatRun({ sessionId: 'session-1', runId: 'run-1' }, { sessionId: 'session-1', runId: 'run-1' }), true);
  assert.equal(isCurrentChatRun({ sessionId: 'session-2', runId: 'run-2' }, { sessionId: 'session-1', runId: 'run-1' }), false);
  assert.equal(isCurrentChatRun({ sessionId: 'session-1', runId: 'run-2' }, { sessionId: 'session-1', runId: 'run-1' }), false);
});
