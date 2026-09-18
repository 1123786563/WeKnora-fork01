import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-019.ts';

test('approval-frozen-revision', async () => {
  const observed = await runProbe({
  "fixture": "open-revision4-current5",
  "fault": "stale-confirm"
});
  assert.deepEqual(observed, {
  "decisionCount": 0,
  "nextAction": "refresh"
});
});
