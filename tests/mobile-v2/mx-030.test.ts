import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-030.ts';

test('logout-device-generation', async () => {
  const observed = await runProbe({
  "fixture": "active-credential-and-device",
  "fault": "late-token-refresh"
});
  assert.deepEqual(observed, {
  "credential": null,
  "activeStreams": 0,
  "oldTokenRewritten": false
});
});
