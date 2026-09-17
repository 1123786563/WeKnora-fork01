import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-012.ts';

test('disk-full-transaction', async () => {
  const observed = await runProbe({
  "fixture": "persisted-seq41",
  "fault": "disk-full-on-seq42"
});
  assert.deepEqual(observed, {
  "cursor": 41,
  "persistedSeqs": [
      41
    ]
});
});
