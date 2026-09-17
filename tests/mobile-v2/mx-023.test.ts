import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-023.ts';

test('connection-revoke-auth-version', async () => {
  const observed = await runProbe({
  "fixture": "auth-version3",
  "fault": "revoke-then-dispatch"
});
  assert.deepEqual(observed, {
  "providerWriteCount": 0,
  "rawSecretFields": []
});
});
