import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-011.ts';

test('late-response-after-switch', async () => {
  const observed = await runProbe({
  "fixture": "tenant-A-to-B",
  "fault": "A-response-arrives-last"
});
  assert.deepEqual(observed, {
    "visibleTenant": "B",
    "oldResponseApplied": false,
    "canceledServerRuns": 0
});
});
