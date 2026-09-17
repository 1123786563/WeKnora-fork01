import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-026.ts';

test('target-not-observable', async () => {
  const observed = await runProbe({
  "fixture": "remote-create-true-observe-false"
});
  assert.deepEqual(observed, {
  "selectedTarget": "platform",
  "admittedRemoteCount": 0
});
});
