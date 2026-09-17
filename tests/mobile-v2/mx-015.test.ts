import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-015.ts';

test('submit-attachment-not-ready', async () => {
  const observed = await runProbe({
  "fixture": "valid-draft-scanning-attachment"
});
  assert.deepEqual(observed, {
  "startCount": 0,
  "draft": "保留这段文本"
});
});
