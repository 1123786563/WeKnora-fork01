import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-017.ts';

test('duplicate-message-event', async () => {
  const observed = await runProbe({
  "fixture": "same-message-seq42-twice"
});
  assert.deepEqual(observed, {
    "renderedMessageCount": 1,
    "text": "你好",
    "happySyncReads": 0
});
});
