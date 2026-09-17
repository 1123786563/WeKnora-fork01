import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-020.ts';

test('typed-interaction-fields', async () => {
  const observed = await runProbe({
  "fixture": "budget-question-connection-tool"
});
  assert.deepEqual(observed, {
  "toolBudgetFieldCount": 0,
  "questionAnswer": "用户编辑后的回答",
  "connectionState": "authorizing"
});
});
