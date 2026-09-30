import test from 'node:test';
import assert from 'node:assert/strict';
import { deliveryViewOf, type DeliveryRemoteRecord } from './delivery-view.ts';

const base: DeliveryRemoteRecord = {
  id: 'dlv-1', taskId: 's-1', runId: 'run-1', state: 'delivered',
  repo: 'octocat/hello', baselineSha: 'b'.repeat(40), branch: 'weknora/task/s-1',
  commitSha: 'c1', prNumber: 7, prUrl: 'https://github.com/octocat/hello/pull/7',
  remoteLogin: 'octocat', actionId: 'act-1', actionState: 'succeeded',
  digest: 'd1', approver: 'u1', files: 2,
  createdAt: '2026-09-24T00:00:00Z', updatedAt: '2026-09-24T00:01:00Z',
};

test('delivered view carries receipts and needs no attention', () => {
  const view = deliveryViewOf(base);
  assert.equal(view.state, 'delivered');
  assert.equal(view.prUrl, 'https://github.com/octocat/hello/pull/7');
  assert.equal(view.approver, 'u1');
  assert.equal(view.attention, false);
});

test('prepared/pushed/unknown states demand owner attention', () => {
  for (const state of ['prepared', 'pushed', 'unknown'] as const) {
    assert.equal(deliveryViewOf({ ...base, state }).attention, true, state);
  }
  assert.equal(deliveryViewOf({ ...base, state: 'dispatched' }).attention, false);
  assert.equal(deliveryViewOf({ ...base, state: 'failed', failure: 'pr: 422' }).attention, false);
});

test('partial push keeps the commit receipt without pr fields', () => {
  const view = deliveryViewOf({ ...base, state: 'pushed', prNumber: undefined, prUrl: undefined });
  assert.equal(view.commitSha, 'c1');
  assert.equal(view.prNumber, undefined);
});
