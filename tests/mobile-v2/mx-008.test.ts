import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-008.ts';

test('sheet-focus-cancel', async () => {
  const observed = await runProbe({
  "fixture": "mounted-native-sheet",
  "fault": "dismiss-before-decision"
});
  assert.deepEqual(observed, {
    "decisionCount": 0,
    "focusTarget": "trigger"
});
});
