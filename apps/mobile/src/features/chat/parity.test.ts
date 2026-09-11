import assert from 'node:assert/strict';
import test from 'node:test';
import { initialChatStreamState, reduceChatStream } from '@weknora/domain/chat/reducer';
import { replayMobileChatEvents, selectIncompleteAssistant, selectReferenceGroups, shouldRenderPendingUser } from './parity.ts';

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

test('mobile does not duplicate a user message after stream recovery reloads history', () => {
  const messages = [{ id: 'u', session_id: 's', role: 'user' as const, content: 'hello' }];
  assert.equal(shouldRenderPendingUser(messages, 'hello'), false);
  assert.equal(shouldRenderPendingUser(messages, 'new draft'), true);
});
