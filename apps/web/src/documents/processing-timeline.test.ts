import assert from 'node:assert/strict';
import test from 'node:test';

import { startProcessingTimeline } from './processing-timeline.ts';

function fakeTimer() {
  let handler: (() => void) | undefined;
  let cleared = false;
  return {
    timer: {
      setInterval(next: () => void, _ms: number) { handler = next; return handle; },
      clearInterval(handleToClear: unknown) { if (handleToClear === handle) cleared = true; },
    },
    async tick() { await handler?.(); },
    get cleared() { return cleared; },
  };
}
const handle = Symbol('handle');

async function flushTimelineTick(): Promise<void> {
  await new Promise<void>((resolve) => setImmediate(resolve));
}

test('fetches immediately, then polls spans every 2s while processing', async () => {
  const fake = fakeTimer();
  const fetches: string[] = [];
  const updates: string[][] = [];
  let status = 'processing';
  const subscription = startProcessingTimeline({
    documentId: 'doc-1',
    intervalMs: 2000,
    getSpans: async (id) => {
      fetches.push(id);
      return { parse_status: status, trace: { name: 'root', children: [{ name: 'docreader', status: 'completed' }] } };
    },
    onUpdate: (steps) => updates.push(steps.map((step) => step.stage + ':' + step.state)),
    timer: fake.timer,
  });
  await flushTimelineTick();
  // Vue fetches on mount, then its permanent interval fetches while active.
  await fake.tick(); await fake.tick();
  assert.deepEqual(fetches, ['doc-1', 'doc-1', 'doc-1']);
  assert.ok(updates[0]![0]!.startsWith('docreader:done'));
  // Document completes: the next poll reports the terminal status, fires
  // onStop semantics through updates, and later ticks quiesce to no-ops.
  status = 'completed';
  await fake.tick();
  assert.deepEqual(fetches, ['doc-1', 'doc-1', 'doc-1', 'doc-1']);
  assert.equal(subscription.stopped, false, 'the subscription remains mounted after a terminal update');
  assert.equal(fake.cleared, false, 'only unmount clears Vue\'s permanent interval');
  await fake.tick();
  assert.equal(fetches.length, 4, 'terminal ticks quiesce without another request');
  subscription.stop();
  assert.ok(fake.cleared);
  await fake.tick();
  assert.equal(fetches.length, 4, 'no polls after stop');
});

test('reports a terminal failure once while retaining the interval until unmount', async () => {
  const fake = fakeTimer();
  const stops: string[] = [];
  let status = 'pending';
  const subscription = startProcessingTimeline({
    documentId: 'doc-2',
    intervalMs: 2000,
    getSpans: async () => ({ parse_status: status }),
    onStop: (spans) => stops.push(String(spans?.parse_status)),
    timer: fake.timer,
  });
  await flushTimelineTick();
  assert.deepEqual(stops, []);
  status = 'failed';
  await fake.tick();
  assert.deepEqual(stops, ['failed']);
  assert.equal(subscription.stopped, false);
  assert.equal(fake.cleared, false);
  // Terminal: the permanent interval stays alive, but no later tick fetches
  // or re-fires onStop.
  await fake.tick();
  assert.deepEqual(stops, ['failed']);
  subscription.stop();
});

test('polling errors are surfaced without killing the interval', async () => {
  const fake = fakeTimer();
  const errors: unknown[] = [];
  let fail = true;
  const subscription = startProcessingTimeline({
    documentId: 'doc-3',
    intervalMs: 2000,
    getSpans: async () => { if (fail) throw new Error('boom'); return { parse_status: 'completed' }; },
    onError: (error) => errors.push(error),
    timer: fake.timer,
  });
  await flushTimelineTick();
  assert.equal(errors.length, 1);
  fail = false;
  await fake.tick();
  assert.equal(errors.length, 1, 'recovered on the next tick');
  subscription.stop();
});
