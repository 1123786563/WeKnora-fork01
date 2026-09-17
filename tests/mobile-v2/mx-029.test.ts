import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-029.ts';

test('voice-interrupt-not-cancel', async () => {
  const observed = await runProbe({
  "fixture": "active-voice-active-run",
  "fault": "interrupt-output"
});
  assert.deepEqual(observed, {
  "mediaOutputStopped": true,
  "runCancelCount": 0
});
});
