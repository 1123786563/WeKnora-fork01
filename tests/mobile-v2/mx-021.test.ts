import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-021.ts';

test('notification-cross-account', async () => {
  const observed = await runProbe({
  "fixture": "device-A-logout-B-login",
  "fault": "late-A-intent"
});
  assert.deepEqual(observed, {
  "deliveredToB": 0,
  "approvalCount": 0
});
});
