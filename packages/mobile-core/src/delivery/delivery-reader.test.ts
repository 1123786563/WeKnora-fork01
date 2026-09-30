import test from 'node:test';
import assert from 'node:assert/strict';
import { createDeliveryReader, DeliveryReaderError, type DeliveryRemote, type DeliveryRemoteRecord } from './delivery-reader.ts';
import { createScenarioDeliveryRemote } from './in-memory-delivery-remote.ts';

const record: DeliveryRemoteRecord = {
  id: 'dlv-1', taskId: 's-1', runId: 'run-1', state: 'prepared',
  repo: 'octocat/hello', baselineSha: 'b'.repeat(40), branch: 'weknora/task/s-1',
  actionId: 'act-1', actionState: 'awaiting_approval', digest: 'd1', files: 1,
  createdAt: '2026-09-24T00:00:00Z', updatedAt: '2026-09-24T00:00:00Z',
};

import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

function mintLease(): { lease: ScopeLease; revoke: () => void } {
  const internal = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.com', userId: 'u1', tenantId: 't1' });
  return { lease: internal.asScopeLease(), revoke: () => internal.revoke() };
}

test('read projects the record under a live lease', async () => {
  const { lease } = mintLease();
  const reader = createDeliveryReader({ remote: createScenarioDeliveryRemote([{ runId: 'run-1', record }]), lease: () => lease });
  const view = await reader.read('run-1');
  assert.equal(view?.state, 'prepared');
  assert.equal(view?.attention, true);
});

test('read without a lease fails closed with DELIVERY_SCOPE_CHANGED', async () => {
  const reader = createDeliveryReader({ remote: createScenarioDeliveryRemote([{ runId: 'run-1', record }]), lease: () => undefined });
  await assert.rejects(() => reader.read('run-1'), (error: unknown) => error instanceof DeliveryReaderError && error.code === 'DELIVERY_SCOPE_CHANGED');
});

test('absent delivery resolves to undefined, remote failure rejects as DELIVERY_BACKEND', async () => {
  const { lease } = mintLease();
  const absent = createDeliveryReader({ remote: createScenarioDeliveryRemote([{ runId: 'run-1', record: null }]), lease: () => lease });
  assert.equal(await absent.read('run-1'), undefined);

  const failing: DeliveryRemote = { delivery: async () => { throw new Error('boom'); } };
  const reader = createDeliveryReader({ remote: failing, lease: () => lease });
  await assert.rejects(() => reader.read('run-1'), (error: unknown) => error instanceof DeliveryReaderError && error.code === 'DELIVERY_BACKEND');
});

test('a late lease revocation drops the in-flight result', async () => {
  const { lease, revoke } = mintLease();
  const slow: DeliveryRemote = {
    delivery: async () => {
      revoke(); // 远端结果返回前 lease 被撤销
      return record;
    },
  };
  const reader = createDeliveryReader({ remote: slow, lease: () => lease });
  await assert.rejects(() => reader.read('run-1'), (error: unknown) => error instanceof DeliveryReaderError && error.code === 'DELIVERY_SCOPE_CHANGED');
});
