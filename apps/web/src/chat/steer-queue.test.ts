import assert from 'node:assert/strict';
import test from 'node:test';

import {
  clearSteerQueue,
  dropSteerItem,
  enqueueSteerItem,
  failSteerItem,
  markSteerAwaitingIdleSend,
  nextSteerIdleSend,
  settleSteerItem,
  steerQueueChips,
  syncSteerQueueFromServer,
  type WebSteerQueueItem,
} from './steer-queue.ts';

/*
 * R473-A2 — Vue steer queue parity (frontend/src/views/chat/index.vue steerQueue):
 * a follow-up queued while an agent turn runs is tracked locally so the
 * composer can show one chip per pending after-message (Input-field.vue
 * .steer-queue) with promote/remove/retry affordances.
 */

test('enqueueSteerItem appends a pending chip item keyed by the client steer id', () => {
  const queue = enqueueSteerItem([], { steerId: 'steer-client-1', content: '更严格一点' });
  assert.deepEqual(queue, [{ steerId: 'steer-client-1', content: '更严格一点', status: 'pending' }]);
});

test('settleSteerItem flips pending to queued and rebases onto the server steer id', () => {
  // Vue handleSteerMsg: queued.steer_id = serverId once POST /steer resolves.
  let queue = enqueueSteerItem([], { steerId: 'steer-client-1', content: 'a' });
  queue = settleSteerItem(queue, 'steer-client-1', 'steer-server-9');
  assert.deepEqual(queue, [{ steerId: 'steer-server-9', content: 'a', status: 'queued' }]);
});

test('settleSteerItem without a server id keeps the client id (older backends)', () => {
  let queue = enqueueSteerItem([], { steerId: 'steer-client-1', content: 'a' });
  queue = settleSteerItem(queue, 'steer-client-1', undefined);
  assert.equal(queue[0]?.steerId, 'steer-client-1');
  assert.equal(queue[0]?.status, 'queued');
});

test('failSteerItem keeps the chip for the retry affordance (Vue item.failed)', () => {
  let queue = enqueueSteerItem([], { steerId: 's1', content: 'a' });
  queue = failSteerItem(queue, 's1');
  assert.deepEqual(queue, [{ steerId: 's1', content: 'a', status: 'failed' }]);
});

test('dropSteerItem removes a consumed, injected or cancelled item', () => {
  let queue = enqueueSteerItem(enqueueSteerItem([], { steerId: 's1', content: 'a' }), { steerId: 's2', content: 'b' });
  queue = dropSteerItem(queue, 's1');
  assert.deepEqual(queue.map((item) => item.steerId), ['s2']);
});

test('clearSteerQueue empties the queue (Vue stop confirmed / session switch)', () => {
  const queue = enqueueSteerItem([], { steerId: 's1', content: 'a' });
  assert.deepEqual(clearSteerQueue(), []);
  void queue;
});

/*
 * Consumption semantics (Vue flushSteerAfterTurn): after the running turn
 * completes, an item the server could not queue (new_run while still
 * streaming) is sent locally, one per completed turn — the first
 * awaiting-idle-send item goes out, the rest wait for the next boundary.
 */
test('nextSteerIdleSend returns the first non-failed awaiting item', () => {
  let queue: WebSteerQueueItem[] = [
    { steerId: 's1', content: 'a', status: 'queued' },
    { steerId: 's2', content: 'b', status: 'queued', awaitingIdleSend: true },
    { steerId: 's3', content: 'c', status: 'queued', awaitingIdleSend: true },
    { steerId: 's4', content: 'd', status: 'failed', awaitingIdleSend: true },
  ];
  assert.equal(nextSteerIdleSend(queue)?.steerId, 's2');
  queue = dropSteerItem(queue, 's2');
  assert.equal(nextSteerIdleSend(queue)?.steerId, 's3');
});

test('markSteerAwaitingIdleSend flags a queued item for local dispatch', () => {
  let queue = enqueueSteerItem([], { steerId: 's1', content: 'a' });
  queue = markSteerAwaitingIdleSend(queue, 's1');
  assert.equal(nextSteerIdleSend(queue)?.steerId, 's1');
});

/*
 * Server sync (Vue hydrateSteerQueue): list /steer owns the backlog that moved
 * into the follow-up run; those items leave the queue while locally failed
 * entries (never accepted server-side) survive with their retry chip.
 */
test('syncSteerQueueFromServer adopts server items and preserves failed + awaiting + pending locals', () => {
  let queue: WebSteerQueueItem[] = [
    { steerId: 's1', content: 'claimed by follow-up run', status: 'queued' },
    { steerId: 's2', content: 'failed enqueue', status: 'failed' },
    { steerId: 's3', content: 'idle send', status: 'queued', awaitingIdleSend: true },
    { steerId: 's4', content: 'post still in flight', status: 'pending' },
  ];
  queue = syncSteerQueueFromServer(queue, [
    { steer_id: 's5', content: 'still queued server-side' },
  ]);
  assert.deepEqual(queue.map((item) => item.steerId), ['s5', 's2', 's3', 's4']);
  assert.equal(queue[0]?.status, 'queued');
});

test('steerQueueChips projects every queue entry for the composer strip', () => {
  const chips = steerQueueChips([
    { steerId: 's1', content: 'queued', status: 'queued' },
    { steerId: 's2', content: 'failed', status: 'failed' },
  ]);
  assert.deepEqual(chips, [
    { steerId: 's1', content: 'queued', status: 'queued' },
    { steerId: 's2', content: 'failed', status: 'failed' },
  ]);
});
