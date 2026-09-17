import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-006.ts';

test('lost-ack-restart', async () => {
  const observed = await runProbe({
  "fixture": "same-request-and-input",
  "fault": "ack-drop-process-restart"
});
  assert.deepEqual(observed, {
    "startCount": 1,
    "lookupRequestId": "request-original",
    "runId": "run-original"
});
});
