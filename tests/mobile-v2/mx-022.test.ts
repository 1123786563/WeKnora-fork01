import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-022.ts';

test('knowledge-access-revoked', async () => {
  const observed = await runProbe({
  "fixture": "cached-knowledge-title",
  "fault": "server-403"
});
  assert.deepEqual(observed, {
  "visibleSensitiveFields": [],
  "canAskWithKnowledge": false
});
});
