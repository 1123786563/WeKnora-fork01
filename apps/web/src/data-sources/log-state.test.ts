import assert from 'node:assert/strict';
import test from 'node:test';
import { isSyncRunning, mergeSyncLogs } from './log-state.ts';

test('replaces the first log page and appends later pages without duplicates', () => {
  const first = [{ id: 'a' }, { id: 'b' }];
  assert.deepEqual(mergeSyncLogs([], first, true), first);
  assert.deepEqual(mergeSyncLogs(first, [{ id: 'b' }, { id: 'c' }], false), [{ id: 'a' }, { id: 'b' }, { id: 'c' }]);
});

test('matches Vue running-state polling semantics from the latest log', () => {
  assert.equal(isSyncRunning({ latest_sync_log: { id: 'run-1', status: 'running' } }), true);
  assert.equal(isSyncRunning({ latest_sync_log: { id: 'done-1', status: 'success' } }), false);
  assert.equal(isSyncRunning({ latest_sync_log: undefined }), false, 'a stale source status must not trigger polling');
});
