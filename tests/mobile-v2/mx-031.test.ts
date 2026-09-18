import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-031.ts';

test('usage-finality', async () => {
  const observed = await runProbe({
  "fixture": "reserved120-pending38-settled1260"
});
  assert.deepEqual(observed, {
  "reserved": 120,
  "pending": 38,
  "settled": 1260,
  "paymentRequests": 0
});
});
