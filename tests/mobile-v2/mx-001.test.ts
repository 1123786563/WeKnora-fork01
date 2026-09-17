import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-001.ts';

test('inventory-coverage', async () => {
  const observed = await runProbe({
  "fixture": "current-checkout-and-G01-G12"
});
  assert.deepEqual(observed, {
    "unownedGaps": [],
    "duplicateOwners": []
});
});
