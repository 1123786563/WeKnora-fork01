import assert from 'node:assert/strict';
import test from 'node:test';

import { buildChatStreamRequest, consumeChatStream, createServerSentEventParser, parseChatEvent } from './stream.ts';

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
  const controller = new AbortController();
  assert.equal(buildChatStreamRequest({ sessionId: 's-1', body: {}, signal: controller.signal }).signal, controller.signal);
});

test('rejects non-object chat SSE payloads', () => {
  assert.throws(() => parseChatEvent({ data: '[]' }), /Invalid chat SSE event/);
});

test('consumes a text stream through the shared request boundary', async () => {
  const events: string[] = [];
  await consumeChatStream(async (request) => {
    assert.equal(request.path, '/api/v1/knowledge-chat/s-1');
    return 'id: e1\ndata: {"response_type":"answer","content":"hi"}\n\ndata: {"response_type":"complete"}\n';
  }, { sessionId: 's-1', body: { query: 'hello' } }, (event) => events.push(String(event.response_type)));
  assert.deepEqual(events, ['answer', 'complete']);
});

test('builds resumable continue and stop calls with cancellation boundaries', async () => {
  const requests: Array<{ method: string; path: string; signal?: AbortSignal }> = [];
  const controller = new AbortController();
  const api = (await import('../client.ts')).createWeKnoraClient({
    baseURL: 'https://weknora.test',
    transport: {
      async send(request) {
        requests.push({ method: request.method, path: new URL(request.url).pathname + new URL(request.url).search, signal: request.signal });
        if (request.method === 'POST') return { status: 200, headers: {}, body: { success: true } };
        return { status: 200, headers: {}, body: 'id: e1\ndata: {"response_type":"complete"}\n' };
      },
    },
  });
  const events: string[] = [];
  await api.chat.continueStream('session/a', 'message/b', (event) => events.push(String(event.response_type)), controller.signal);
  await api.chat.stop('session/a', 'message/b', controller.signal);
  assert.deepEqual(events, ['complete']);
  assert.equal(requests[0]?.method, 'GET');
  assert.equal(requests[0]?.path, '/api/v1/sessions/continue-stream/session%2Fa?message_id=message%2Fb');
  assert.equal(requests[0]?.signal?.aborted, false);
  assert.equal(requests[1]?.path, '/api/v1/sessions/session%2Fa/stop');
});
