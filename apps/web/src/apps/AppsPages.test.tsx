import assert from 'node:assert/strict';
import test from 'node:test';

import { appDigest, appRisk, appRows, appErrorMessage, installationState } from './model.ts';

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

test('matches the Vue AppsView semantic tags and digest preview', () => {
  assert.deepEqual(appRisk('read'), { label: '只读', tone: 'success' });
  assert.deepEqual(appRisk('write'), { label: '写入', tone: 'warning' });
  assert.deepEqual(appRisk('delete'), { label: '删除', tone: 'danger' });
  assert.deepEqual(appRisk('future'), { label: 'future', tone: 'neutral' });
  assert.equal(appDigest('1234567890123456'), '123456789012…');
  assert.deepEqual(installationState('active'), { label: '活跃', tone: 'success' });
  assert.deepEqual(installationState('disabled'), { label: '已停用', tone: 'neutral' });
  assert.deepEqual(installationState('paused'), { label: '状态：paused', tone: 'neutral' });
});
