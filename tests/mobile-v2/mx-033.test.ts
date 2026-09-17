import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-033.ts';

test('crash-recovery-budget', async () => {
  const observed = await runProbe({
  "fixture": "side-effect-succeeded",
  "fault": "crash-before-local-settlement"
});
  assert.deepEqual(observed, {
  "providerWriteCount": 1,
  "settlementCount": 1
});
});
