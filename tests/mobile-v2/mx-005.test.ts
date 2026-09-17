import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-005.ts';

test('concurrent-decisions', async () => {
  const observed = await runProbe({
  "fixture": "same-interaction-revision4",
  "fault": "two-concurrent-approve"
});
  assert.deepEqual(observed, {
    "acceptedCount": 1,
    "conflictCount": 1
});
});
