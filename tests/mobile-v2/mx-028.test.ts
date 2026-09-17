import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-028.ts';

test('edited-transcript', async () => {
  const observed = await runProbe({
  "fixture": "initial-transcript",
  "fault": "edit-before-confirm"
});
  assert.deepEqual(observed, {
  "draft": "用户最终编辑的文本",
  "submittedMessages": 0
});
});
