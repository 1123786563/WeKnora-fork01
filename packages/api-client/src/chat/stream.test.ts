import assert from 'node:assert/strict';
import test from 'node:test';

import { buildChatStreamRequest, createServerSentEventParser, parseChatEvent } from './stream.ts';

test('parses split and multi-line SSE data frames', () => {
  const events: Array<{ id?: string; event?: string; data: string }> = [];
  const parser = createServerSentEventParser((event) => events.push(event));
  parser.push('id: 7\nevent: message\ndata: {"response_type":"answer",\n');
  parser.push('data: "content":"hi"}\n\n: keepalive\n\n');
  parser.finish();
  assert.deepEqual(events, [{ id: '7', event: 'message', data: '{"response_type":"answer",\n"content":"hi"}' }]);
  assert.equal(parseChatEvent(events[0]!).event_id, '7');
});

test('builds knowledge and agent stream paths with resumable event id', () => {
  assert.deepEqual(buildChatStreamRequest({ sessionId: 'session/a', body: { query: 'hi' }, lastEventId: 'evt-3' }), {
    method: 'POST',
    path: '/api/v1/knowledge-chat/session%2Fa',
    headers: { accept: 'text/event-stream', 'Last-Event-ID': 'evt-3' },
    body: { query: 'hi' },
  });
  assert.equal(buildChatStreamRequest({ sessionId: 's-1', mode: 'agent', body: {} }).path, '/api/v1/agent-chat/s-1');
});

test('rejects non-object chat SSE payloads', () => {
  assert.throws(() => parseChatEvent({ data: '[]' }), /Invalid chat SSE event/);
});
