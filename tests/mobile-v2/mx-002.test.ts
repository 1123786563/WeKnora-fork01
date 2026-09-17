import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-002.ts';

test('native-version-compat', async () => {
  const observed = await runProbe({
  "fixture": "locked-expo55-baseline"
});
  assert.deepEqual(observed, {
    "rendererMatchesReact": true,
    "nativeBuildPlatforms": [
      "ios",
      "android"
    ]
});
});
