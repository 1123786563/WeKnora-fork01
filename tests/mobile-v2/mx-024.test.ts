import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-024.ts';

test('artifact-link-expired', async () => {
  const observed = await runProbe({
  "fixture": "artifact-version1",
  "fault": "expired-url-then-permission-revoked"
});
  assert.deepEqual(observed, {
  "shareCount": 0,
  "persistentSignedUrls": []
});
});
