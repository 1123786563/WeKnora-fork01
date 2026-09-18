import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-036.ts';

test('release-blocked-subgate', async () => {
  const observed = await runProbe({
  "fixture": "core-pass-remote-blocked-env"
});
  assert.deepEqual(observed, {
  "core": "releasable",
  "remote": "blocked-env",
  "globalAllPassed": false
});
});
