import assert from 'node:assert/strict';
import test from 'node:test';

import { appRows, appErrorMessage } from './model.ts';

test('normalizes standard app list envelopes without dropping nested rows', () => {
  assert.deepEqual(appRows({ data: { items: [{ id: 'a-1' }] } }), [{ id: 'a-1' }]);
  assert.deepEqual(appRows({ data: { rows: [{ id: 'a-2' }] } }), [{ id: 'a-2' }]);
  assert.deepEqual(appRows({ items: [{ id: 'a-3' }] }), [{ id: 'a-3' }]);
  assert.deepEqual(appRows([{ id: 'a-4' }]), [{ id: 'a-4' }]);
});

test('preserves server errors and gives non-error failures a stable message', () => {
  assert.equal(appErrorMessage(new Error('provider unavailable')), 'provider unavailable');
  assert.equal(appErrorMessage('failure'), '应用页面加载失败');
});
