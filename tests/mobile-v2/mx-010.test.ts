import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-010.ts';

test('cold-start-scope', async () => {
  const observed = await runProbe({
  "fixture": "valid-credential-missing-scope"
});
  assert.deepEqual(observed, {
    "userId": "u1",
    "tenantId": "t1",
    "credentialInOrdinaryStorage": false
});
});
