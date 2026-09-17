import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-007.ts';

test('token-theme-parity', async () => {
  const observed = await runProbe({
  "fixture": "tokens.json"
});
  assert.deepEqual(observed, {
    "missingThemeKeys": [],
    "webNativeColorDiff": []
});
});
