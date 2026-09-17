import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from '../probes/mx-034.ts';

test('native-core-e2e', async () => {
  const observed = await runProbe({
  "fixture": "controlled-platform-agent-ios-android"
});
  assert.deepEqual(observed, {
  "verifiedPlatforms": [
      "ios",
      "android"
    ],
  "duplicateRunCount": 0
});
});
