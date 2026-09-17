import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-003.ts';

test('unknown-capability', async () => {
  const observed = await runProbe({
  "fixture": "unknown-event-and-unavailable-cancel"
});
  assert.deepEqual(observed, {
    "unknownType": "future.event",
    "canCancel": false
});
});
