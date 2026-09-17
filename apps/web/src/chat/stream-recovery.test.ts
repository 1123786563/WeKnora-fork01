import assert from 'node:assert/strict';
import test from 'node:test';

import { ChatStreamApplicationError, feedWithLastEventId, isChatStreamApplicationError, resumeStreamOptions, streamFailureMessage, type LastEventIdHolder } from './stream-recovery.ts';

/*
 * Vue stream failure copy (frontend/src/api/chat/streame.ts): a failed
 * handshake throws `HTTP ${status}` and onerror surfaces
 * `${error.streamFailed}: ${message}` — the localized prefix plus the HTTP
 * status, e.g. 「流式连接失败: HTTP 404」. The api-client raises
 * `Chat stream failed with HTTP 404`, so the React wrapper extracts the
 * status instead of embedding the whole English sentence.
 */
test('streamFailureMessage surfaces the localized prefix with the HTTP status', () => {
  assert.equal(
    streamFailureMessage(new Error('Chat stream failed with HTTP 404'), '流式连接失败'),
    '流式连接失败: HTTP 404',
  );
  assert.equal(
    streamFailureMessage(new Error('HTTP 502'), 'Stream connection failed'),
    'Stream connection failed: HTTP 502',
  );
});

test('streamFailureMessage keeps non-HTTP transport reasons after the prefix like Vue', () => {
  assert.equal(
    streamFailureMessage(new Error('Failed to fetch'), '流式连接失败'),
    '流式连接失败: Failed to fetch',
  );
  assert.equal(streamFailureMessage('socket hang up', '流式连接失败'), '流式连接失败: socket hang up');
});

test('application stream errors are terminal and must not be retried as transport failures', () => {
  const error = new ChatStreamApplicationError('quota exceeded');
  assert.equal(isChatStreamApplicationError(error), true);
  assert.equal(isChatStreamApplicationError(new Error('connection reset')), false);
});

test('resumeStreamOptions attaches the last seen event id once', () => {
  const base = { sessionId: 's1', body: { query: 'hi' } };
  const retry = resumeStreamOptions(base, 'evt-7');
  assert.deepEqual(retry, { sessionId: 's1', body: { query: 'hi' }, lastEventId: 'evt-7' });
  // No second retry: the retry already carries a Last-Event-ID.
  assert.equal(resumeStreamOptions(retry!, 'evt-9'), null);
});

test('resumeStreamOptions returns null without a seen event id', () => {
  assert.equal(resumeStreamOptions({ sessionId: 's1', body: {} }, undefined), null);
  assert.equal(resumeStreamOptions({ sessionId: 's1', body: {} }, ''), null);
});

test('feedWithLastEventId records the newest event id and forwards every event', () => {
  const seen: string[] = [];
  const holder: LastEventIdHolder = {};
  const feed = feedWithLastEventId((event) => { seen.push(event.event_id ?? ''); }, holder);
  feed({ event_id: 'evt-1' });
  feed({ event_id: 'evt-2' });
  feed({ event_id: '' });
  feed({});
  assert.deepEqual(seen, ['evt-1', 'evt-2', '', '']);
  assert.equal(holder.id, 'evt-2');
});

test('a mid-flight failure resumes once from the last event id', async () => {
  // Fake transport: the first POST fails after two events; the resume request
  // must carry Last-Event-ID and receive the remaining events.
  const received: string[] = [];
  const headersSeen: (string | undefined)[] = [];
  let calls = 0;
  const stream = async (options: { lastEventId?: string }, onEvent: (event: { event_id?: string }) => void): Promise<void> => {
    calls += 1;
    headersSeen.push(options.lastEventId);
    const events = calls === 1
      ? [{ event_id: 'evt-1' }, { event_id: 'evt-2' }]
      : [{ event_id: 'evt-3' }, { event_id: 'evt-4' }];
    for (const event of events) {
      onEvent(event);
      if (calls === 1 && event.event_id === 'evt-2') throw new Error('connection reset');
    }
  };

  const base: { sessionId: string; body: Record<string, unknown>; lastEventId?: string } = { sessionId: 's1', body: { query: 'hi' } };
  const holder: LastEventIdHolder = {};
  const feed = feedWithLastEventId((event) => { received.push(event.event_id ?? ''); }, holder);
  try {
    await stream(base, feed);
    assert.fail('expected the first stream to fail');
  } catch (cause) {
    assert.match(String(cause), /connection reset/);
  }
  const retry = resumeStreamOptions(base, holder.id);
  assert.ok(retry, 'a resume must be offered after receiving events');
  await stream(retry, feed);

  assert.equal(calls, 2);
  assert.deepEqual(headersSeen, [undefined, 'evt-2']);
  assert.deepEqual(received, ['evt-1', 'evt-2', 'evt-3', 'evt-4']);
  // No further resume: the retry already carried a Last-Event-ID.
  assert.equal(resumeStreamOptions(retry, holder.id), null);
});
