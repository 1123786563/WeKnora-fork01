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

/*
 * R475-A3 — retry delivery parity (Vue handleRetrySteer passes item.delivery):
 * the queue item persists its delivery mode so a failed inject retry stays an
 * inject instead of degrading to 'after'.
 */
test('enqueueSteerItem persists the delivery mode on the pending chip item', () => {
  const queue = enqueueSteerItem([], { steerId: 'steer-client-1', content: '更严格一点' });
  assert.deepEqual(queue, [{ steerId: 'steer-client-1', content: '更严格一点', status: 'pending', delivery: 'after' }]);
  const injectQueue = enqueueSteerItem([], { steerId: 'steer-client-2', content: '现在就注入', delivery: 'inject' });
  assert.deepEqual(injectQueue, [{ steerId: 'steer-client-2', content: '现在就注入', status: 'pending', delivery: 'inject' }]);
});

test('settleSteerItem and failSteerItem keep the delivery mode through transitions', () => {
  let queue = enqueueSteerItem([], { steerId: 'steer-client-1', content: 'a', delivery: 'inject' });
  queue = settleSteerItem(queue, 'steer-client-1', 'steer-server-9');
  assert.equal(queue[0]?.delivery, 'inject');
  queue = failSteerItem(queue, 'steer-server-9');
  assert.equal(queue[0]?.delivery, 'inject', 'a failed inject keeps its delivery for the retry');
  queue = markSteerAwaitingIdleSend(queue, 'steer-server-9');
  assert.equal(queue[0]?.delivery, 'inject');
});

test('enqueueSteerItem appends a pending chip item keyed by the client steer id', () => {
  const queue = enqueueSteerItem([], { steerId: 'steer-client-1', content: '更严格一点' });
  assert.deepEqual(queue, [{ steerId: 'steer-client-1', content: '更严格一点', status: 'pending', delivery: 'after' }]);
});

test('settleSteerItem flips pending to queued and rebases onto the server steer id', () => {
  // Vue handleSteerMsg: queued.steer_id = serverId once POST /steer resolves.
  let queue = enqueueSteerItem([], { steerId: 'steer-client-1', content: 'a' });
  queue = settleSteerItem(queue, 'steer-client-1', 'steer-server-9');
  assert.deepEqual(queue, [{ steerId: 'steer-server-9', content: 'a', status: 'queued', delivery: 'after' }]);
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
  assert.deepEqual(queue, [{ steerId: 's1', content: 'a', status: 'failed', delivery: 'after' }]);
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
    { steerId: 's1', content: 'claimed by follow-up run', status: 'queued', delivery: 'after' },
    { steerId: 's2', content: 'failed enqueue', status: 'failed', delivery: 'inject' },
    { steerId: 's3', content: 'idle send', status: 'queued', awaitingIdleSend: true, delivery: 'after' },
    { steerId: 's4', content: 'post still in flight', status: 'pending', delivery: 'after' },
  ];
  queue = syncSteerQueueFromServer(queue, [
    { steer_id: 's5', content: 'still queued server-side', delivery: 'inject' },
    { steer_id: 's6', content: 'also queued, no delivery field' },
  ]);
  assert.deepEqual(queue.map((item) => item.steerId), ['s5', 's6', 's2', 's3', 's4']);
  assert.equal(queue[0]?.status, 'queued');
  assert.equal(queue[0]?.delivery, 'inject', 'server items keep their delivery (Vue hydrateSteerQueue line 661)');
  assert.equal(queue[1]?.delivery, 'after', 'a missing server delivery normalises to after');
  assert.equal(queue[2]?.delivery, 'inject', 'preserved failed locals keep their delivery');
});

test('steerQueueChips projects only after-delivery entries for the composer strip', () => {
  // Vue index.vue line 163: queuedSteers = steerQueue.filter(item => item.delivery === 'after')
  // — an inject surfaces as an optimistic user bubble, not a chip.
  const chips = steerQueueChips([
    { steerId: 's1', content: 'queued', status: 'queued', delivery: 'after' },
    { steerId: 's2', content: 'failed', status: 'failed', delivery: 'after' },
    { steerId: 's3', content: 'optimistic inject preview', status: 'pending', delivery: 'inject' },
  ]);
  assert.deepEqual(chips, [
    { steerId: 's1', content: 'queued', status: 'queued' },
    { steerId: 's2', content: 'failed', status: 'failed' },
  ]);
});
