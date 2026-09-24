import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileInboxRemote } from './inbox.ts';
import type { ClientRequest } from '../client.ts';

const ORIGIN = 'https://weknora.example.test';

function recorder(responder: (input: ClientRequest) => unknown): { seen: ClientRequest[]; request: (input: ClientRequest) => Promise<unknown> } {
  const seen: ClientRequest[] = [];
  return {
    seen,
    request: async (input: ClientRequest): Promise<unknown> => {
      seen.push(input);
      return responder(input);
    },
  };
}

/** 真实 GET /api/v1/workbench/inbox 响应（internal/handler/session/workbench_inbox.go:53-119）。 */
const INBOX_WIRE = {
  success: true,
  data: {
    items: [
      { notification_id: 'n-1', kind: 'attention', title: '需要你处理', body: '', created_at: '2026-09-24T01:00:00Z', read: false, deep_link: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
      { notification_id: 'n-2', kind: 'budget', title: '预算事件', body: '', created_at: '2026-09-24T02:00:00Z', read: true, deep_link: '' },
    ],
    unread_count: 1,
    next_cursor: '2026-09-24T01:00:00Z',
  },
};
const INBOX_WIRE_END = { success: true, data: { items: [], unread_count: 0, next_cursor: '' } };
const MARK_READ_WIRE = { success: true, data: { notification_id: 'n-1', read: true } };

test('inbox maps wire rows, omits empty deep_link, and normalizes an empty next_cursor', async () => {
  let call = 0;
  const spy = recorder(() => (call += 1) === 1 ? INBOX_WIRE : INBOX_WIRE_END);
  const remote = createMobileInboxRemote({ origin: ORIGIN, request: spy.request });

  const first = await remote.inbox();
  assert.deepEqual(first, {
    items: [
      { notificationId: 'n-1', kind: 'attention', title: '需要你处理', body: '', createdAt: '2026-09-24T01:00:00Z', read: false, deepLink: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
      { notificationId: 'n-2', kind: 'budget', title: '预算事件', body: '', createdAt: '2026-09-24T02:00:00Z', read: true },
    ],
    unreadCount: 1,
    nextCursor: '2026-09-24T01:00:00Z',
  });
  assert.equal(spy.seen[0]!.method, 'GET');
  assert.equal(spy.seen[0]!.path, '/api/v1/workbench/inbox');

  const last = await remote.inbox('2026-09-24T01:00:00Z');
  assert.equal(last.nextCursor, undefined, '空 next_cursor 归一为无下一页');
  assert.equal(spy.seen[1]!.path, '/api/v1/workbench/inbox?cursor=2026-09-24T01%3A00%3A00Z', 'cursor 必须编码进 query');
});

test('markRead posts the notification_id body and expects the read receipt envelope', async () => {
  const spy = recorder(() => MARK_READ_WIRE);
  const remote = createMobileInboxRemote({ origin: ORIGIN, request: spy.request });

  await remote.markRead('n-1');

  assert.equal(spy.seen[0]!.method, 'POST');
  assert.equal(spy.seen[0]!.path, '/api/v1/workbench/inbox/read');
  assert.deepEqual(spy.seen[0]!.body, { notification_id: 'n-1' });
  await assert.rejects(remote.markRead('  '), /notification id/);
});

test('malformed envelopes and origins fail fast', async () => {
  const failing = recorder(() => ({ data: {} }));
  const remote = createMobileInboxRemote({ origin: ORIGIN, request: failing.request });
  await assert.rejects(remote.inbox(), /success/);

  assert.throws(() => createMobileInboxRemote({ origin: 'https://weknora.example.test/api/v1', request: failing.request }), /path/);
  assert.throws(() => createMobileInboxRemote({ origin: 'https://user:pass@weknora.example.test', request: failing.request }), /user info/);
});
