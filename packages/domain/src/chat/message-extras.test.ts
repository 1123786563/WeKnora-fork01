import assert from 'node:assert/strict';
import test from 'node:test';

import { assistantMessageExtras } from './message-extras.ts';

test('rebuilds thinking and tool calls from persisted agent_steps', () => {
  const message = {
    id: 'assistant-1',
    session_id: 'session-1',
    role: 'assistant',
    content: 'final answer',
    agent_steps: [
      { iteration: 0, thought: 'first plan', reasoning_content: '', tool_calls: [{ id: 'call-1', name: 'search_docs', args: {} }] },
      { iteration: 1, thought: 'second plan', reasoning_content: 'deeper reasoning', tool_calls: [] },
    ],
  } as never;
  const extras = assistantMessageExtras(message);
  assert.equal(extras.thinking, 'first plan\n\ndeeper reasoning');
  assert.deepEqual(extras.toolCalls.map((call) => [call.id, call.name]), [['call-1', 'search_docs']]);
  assert.equal(extras.toolCalls[0]?.status, 'completed');
});

test('prefers the live transient fields streamed onto the assistant row', () => {
  const message = {
    id: 'stream-session-1',
    session_id: 'session-1',
    role: 'assistant',
    content: 'partial',
    thinking: 'live thinking',
    tool_calls: [{ id: 'call-live', name: 'run_code', status: 'pending' }],
  } as never;
  const extras = assistantMessageExtras(message);
  assert.equal(extras.thinking, 'live thinking');
  assert.equal(extras.toolCalls[0]?.id, 'call-live');
  assert.equal(extras.toolCalls[0]?.status, 'pending');
});

test('returns empty extras for plain messages', () => {
  const message = { id: 'a', session_id: 's', role: 'assistant', content: 'hi' } as never;
  assert.deepEqual(assistantMessageExtras(message), { thinking: '', toolCalls: [] });
  assert.deepEqual(assistantMessageExtras(undefined as never), { thinking: '', toolCalls: [] });
});
