import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-025.ts';

test('native-file-switch', async () => {
  const observed = await runProbe({
  "fixture": "Android-content-uri",
  "fault": "tenant-switch-during-upload"
});
  assert.deepEqual(observed, {
  "lateResultApplied": false,
  "draftPreserved": true
});
});
