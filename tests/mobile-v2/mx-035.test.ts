import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-035.ts';

test('visual-a11y-matrix', async () => {
  const observed = await runProbe({
  "fixture": "all-pages-light-dark-200-percent-native"
});
  assert.deepEqual(observed, {
  "unreachablePrimaryActions": [],
  "criticalA11yFindings": []
});
});
