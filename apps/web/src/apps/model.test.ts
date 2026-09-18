import assert from 'node:assert/strict';
import test from 'node:test';

import { appDigest, appErrorMessage, appRows } from './model.ts';
import { actionControls } from './actionState.ts';
import { POLL_INTERVAL_MS, POLL_MAX_INTERVAL_MS, pollBackoffDelayMs } from './pollBackoff.ts';
import { formatMessage } from '@weknora/i18n';

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

test('matches the Vue AppsView digest preview rule', () => {
  assert.equal(appDigest('1234567890123456'), '123456789012…');
  assert.equal(appDigest(''), '—');
});

test('uses the Vue actionState contract for approval controls', () => {
  assert.deepEqual(actionControls({ id: 'a', state: 'awaiting_approval', digest: 'd', canApprove: true, canExecute: true }), { approve: true, execute: false, retry: false });
  assert.deepEqual(actionControls({ id: 'a', state: 'awaiting_approval', digest: 'd', canApprove: false, canExecute: true }), { approve: false, execute: false, retry: false });
  assert.deepEqual(actionControls({ id: 'a', state: 'authorized', digest: 'd', canApprove: true, canExecute: true }), { approve: false, execute: true, retry: false });
  assert.deepEqual(actionControls({ id: 'a', state: 'authorized', digest: 'd', canApprove: true, canExecute: false }), { approve: false, execute: false, retry: false });
  assert.deepEqual(actionControls({ id: 'a', state: 'succeeded', digest: 'd', canApprove: true, canExecute: true }), { approve: false, execute: false, retry: false });
  assert.deepEqual(actionControls({ id: 'a', state: 'unknown', digest: 'd', canApprove: true, canExecute: true }), { approve: false, execute: false, retry: false });
});

test('the poll backoff curve matches the Vue pollBackoff module', () => {
  assert.equal(pollBackoffDelayMs(0), POLL_INTERVAL_MS);
  assert.equal(pollBackoffDelayMs(1), POLL_INTERVAL_MS * 2);
  assert.equal(pollBackoffDelayMs(2), POLL_INTERVAL_MS * 4);
  assert.equal(pollBackoffDelayMs(3), POLL_INTERVAL_MS * 8);
  assert.equal(pollBackoffDelayMs(4), POLL_MAX_INTERVAL_MS);
  assert.equal(pollBackoffDelayMs(50), POLL_MAX_INTERVAL_MS);
  assert.equal(pollBackoffDelayMs(-1), POLL_INTERVAL_MS);
});

test('apps labels resolve through the byte-exact generated i18n block', () => {
  assert.equal(formatMessage('zh-CN', 'apps.risk.read'), '只读');
  assert.equal(formatMessage('zh-CN', 'apps.risk.write'), '写入');
  assert.equal(formatMessage('zh-CN', 'apps.risk.send'), '发送');
  assert.equal(formatMessage('zh-CN', 'apps.risk.delete'), '删除');
  assert.equal(formatMessage('en-US', 'apps.risk.read'), 'Read');
  assert.equal(formatMessage('zh-CN', 'apps.common.stateOther', { state: 'paused' }), '状态：paused');
  assert.equal(formatMessage('zh-CN', 'apps.connections.revokeConfirmContent'), '断开后本空间立即失去该连接授权；远端清理可能仍在后台进行。确定断开吗？');
  assert.equal(formatMessage('ja-JP', 'apps.actions.title'), 'アクション承認');
});
