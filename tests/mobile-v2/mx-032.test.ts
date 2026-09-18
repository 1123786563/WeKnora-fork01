import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-032.ts';

test('profile-with-missing-evidence', async () => {
  const observed = await runProbe({
  "fixture": "remote-without-cancel-evidence"
});
  assert.deepEqual(observed, {
  "remoteCapability": "unavailable",
  "reason": "missing_cancel_evidence"
});
});
