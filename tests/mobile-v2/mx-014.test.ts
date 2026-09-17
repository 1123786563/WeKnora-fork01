import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-014.ts';

test('pagination-filter-scope', async () => {
  const observed = await runProbe({
  "fixture": "equal-updated-at-two-pages",
  "fault": "filter-change-late-response"
});
  assert.deepEqual(observed, {
    "duplicateIds": [],
    "appliedFilter": "waiting_user"
});
});
