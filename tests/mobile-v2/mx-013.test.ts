import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-013.ts';

test('overview-owner-scope', async () => {
  const observed = await runProbe({
  "fixture": "two-users-one-tenant"
});
  assert.deepEqual(observed, {
  "visibleRunIds": [
      "owned-run"
    ],
  "perSessionHTTPRequests": 0
});
});
