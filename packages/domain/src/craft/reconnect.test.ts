// C03 craft reconnection rules — tests.
//
// Step 1 of the brief runs the replayAction test FIRST (RED), before the
// module exists. The backoff cases below pin the C03 Step 5 schedule
// (1/2/4/8/15s + jitter, capped at 15s) as a pure function so the controller
// only has to inject and count attempts.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  RECONNECT_BACKOFF_CAP_MS,
  RECONNECT_BACKOFF_STEPS_MS,
  reconnectDelayMs,
  replayAction,
} from './reconnect.ts';

test('event gaps require authoritative reload', () => {
  assert.equal(replayAction(8, 8), 'drop');
  assert.equal(replayAction(8, 10), 'reload');
  assert.equal(replayAction(8, 9), 'apply');
});

test('older and far-future seqs resolve to drop and reload', () => {
  assert.equal(replayAction(5, 4), 'drop');
  assert.equal(replayAction(5, 0), 'drop');
  assert.equal(replayAction(0, 1), 'apply');
  assert.equal(replayAction(3, 100), 'reload');
});

test('backoff follows the 1/2/4/8/15s steps without jitter', () => {
  const noJitter = (): number => 0;
  assert.deepEqual(RECONNECT_BACKOFF_STEPS_MS, [1000, 2000, 4000, 8000, 15000]);
  for (let attempt = 0; attempt < RECONNECT_BACKOFF_STEPS_MS.length; attempt += 1) {
    assert.equal(reconnectDelayMs(attempt, noJitter), RECONNECT_BACKOFF_STEPS_MS[attempt]);
  }
  // Attempts beyond the schedule hold the last step (steady 15s).
  assert.equal(reconnectDelayMs(99, noJitter), 15000);
});

test('jitter never crosses the 15s cap and stays within its step band', () => {
  const maxJitter = (): number => 0.999;
  for (let attempt = 0; attempt < 12; attempt += 1) {
    const delay = reconnectDelayMs(attempt, maxJitter);
    assert.ok(delay <= RECONNECT_BACKOFF_CAP_MS, 'delay must respect the 15s cap');
  }
  // 20% relative jitter: attempt 0 lands in (1000, 1200].
  assert.ok(reconnectDelayMs(0, maxJitter) > 1000 && reconnectDelayMs(0, maxJitter) <= 1200);
  // The capped step clamps jitter to exactly 15s.
  assert.equal(reconnectDelayMs(4, maxJitter), RECONNECT_BACKOFF_CAP_MS);
});

test('negative attempts are treated as the first step', () => {
  assert.equal(reconnectDelayMs(-3, (): number => 0), 1000);
});
