import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-018.ts';

test('cancel-not-stop', async () => {
  const observed = await runProbe({
  "fixture": "cancel-ack-no-process-exit"
});
  assert.deepEqual(observed, {
  "executionStatus": "stop_pending",
  "label": "停止待确认",
  "refundRequested": false
});
});
