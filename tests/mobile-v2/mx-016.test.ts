import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-016.ts';

test('unavailable-agent', async () => {
  const observed = await runProbe({
  "fixture": "coding-driver-unavailable"
});
  assert.deepEqual(observed, {
  "selectedAgent": "general",
  "unavailableReason": "driver_unavailable"
});
});
