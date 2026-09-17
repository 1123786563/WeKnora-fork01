import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-009.ts';

test('product-route-no-happy', async () => {
  const observed = await runProbe({
  "fixture": "product-scope-without-happy-sync"
});
  assert.deepEqual(observed, {
    "visibleTabs": [
      "工作台",
      "会话",
      "资源",
      "我的"
    ],
    "happyAuthRequests": 0
});
});
