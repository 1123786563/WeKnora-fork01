import test from 'node:test';
import assert from 'node:assert/strict';
import { createDeliveryRecovery, DeliveryRecoveryError, type DeliveryRecoveryRemote } from './delivery-recovery.ts';
import type { DeliveryRemote, DeliveryRemoteRecord } from './delivery-reader.ts';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

function mintLease(): { lease: ScopeLease; revoke: () => void } {
  const internal = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.com', userId: 'u1', tenantId: 't1' });
  return { lease: internal.asScopeLease(), revoke: () => internal.revoke() };
}

function recordWith(state: DeliveryRemoteRecord['state']): DeliveryRemoteRecord {
  return {
    id: 'dlv-1', taskId: 's-1', runId: 'run-1', state,
    repo: 'octocat/hello', baselineSha: 'b'.repeat(40), branch: 'weknora/task/s-1',
    commitSha: 'c1', actionId: 'act-1', actionState: 'dispatched', digest: 'd1', files: 1,
    createdAt: '2026-09-24T00:00:00Z', updatedAt: '2026-09-24T00:00:30Z',
  };
}

function harness(state: DeliveryRemoteRecord['state']) {
  const calls: string[] = [];
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { calls.push('delivery'); return recordWith(state); },
    async dispatchDelivery() { calls.push('dispatchDelivery'); return recordWith('delivered'); },
    async resolveDelivery() { calls.push('resolveDelivery'); return recordWith('pushed'); },
  };
  const { lease } = mintLease();
  let revoked = false;
  return {
    calls,
    recover: createDeliveryRecovery({ remote, lease: () => (revoked ? undefined : lease) }).recover,
    setRevoked(value: boolean) { revoked = value; },
  };
}

test('recover routes pushed to dispatch', async () => {
  const h = harness('pushed');
  assert.equal((await h.recover({ runId: 'run-1', deliveryId: 'dlv-1' })).state, 'delivered');
  assert.deepEqual(h.calls, ['delivery', 'dispatchDelivery']);
});

test('recover routes unknown to resolve', async () => {
  const h = harness('unknown');
  assert.equal((await h.recover({ runId: 'run-1', deliveryId: 'dlv-1' })).state, 'pushed');
  assert.deepEqual(h.calls, ['delivery', 'resolveDelivery']);
});

test('delivered is idempotent with no write', async () => {
  const h = harness('delivered');
  assert.equal((await h.recover({ runId: 'run-1', deliveryId: 'dlv-1' })).state, 'delivered');
  assert.deepEqual(h.calls, ['delivery']);
});

test('prepared, dispatched, and failed conflict without writes', async () => {
  for (const state of ['prepared', 'dispatched', 'failed'] as const) {
    const h = harness(state);
    await assert.rejects(() => h.recover({ runId: 'run-1', deliveryId: 'dlv-1' }), (error: unknown) =>
      error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_STATE_CONFLICT' && error.message.includes(state));
    assert.deepEqual(h.calls, ['delivery']);
  }
});

test('missing delivery is invalid input', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return null; },
    async dispatchDelivery() { throw new Error('unexpected'); },
    async resolveDelivery() { throw new Error('unexpected'); },
  };
  const { lease } = mintLease();
  await assert.rejects(() => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_INVALID_INPUT');
});

test('missing lease rejects before network', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { throw new Error('unexpected'); },
    async dispatchDelivery() { throw new Error('unexpected'); },
    async resolveDelivery() { throw new Error('unexpected'); },
  };
  await assert.rejects(() => createDeliveryRecovery({ remote, lease: () => undefined }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_SCOPE_CHANGED');
});

test('revocation during write drops late result', async () => {
  const internal = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.com', userId: 'u1', tenantId: 't1' });
  const lease = internal.asScopeLease();
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return recordWith('pushed'); },
    async dispatchDelivery() { internal.revoke(); return recordWith('delivered'); },
    async resolveDelivery() { throw new Error('unexpected'); },
  };
  await assert.rejects(() => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_SCOPE_CHANGED');
});

test('409 conflict translates for both supported ApiError shapes', async () => {
  for (const shape of [
    Object.assign(new Error('409'), { status: 409, code: 'code_delivery_state_conflict' }),
    Object.assign(new Error('409'), { status: 409, body: { code: 'code_delivery_state_conflict' } }),
    Object.assign(new Error('conflict'), { code: 'code_delivery_state_conflict' }),
    Object.assign(new Error('conflict'), { body: { code: 'code_delivery_state_conflict' } }),
  ]) {
    const remote: DeliveryRemote & DeliveryRecoveryRemote = {
      async delivery() { return recordWith('pushed'); },
      async dispatchDelivery() { throw shape; },
      async resolveDelivery() { throw new Error('unexpected'); },
    };
    const { lease } = mintLease();
    await assert.rejects(() => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
      (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_STATE_CONFLICT');
  }
});

test('other backend failures translate to backend error', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return recordWith('pushed'); },
    async dispatchDelivery() { throw new Error('network down'); },
    async resolveDelivery() { throw new Error('unexpected'); },
  };
  const { lease } = mintLease();
  await assert.rejects(() => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_BACKEND');
});

test('revocation during read rejects before any write', async () => {
  const internal = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.com', userId: 'u1', tenantId: 't1' });
  const lease = internal.asScopeLease();
  const calls: string[] = [];
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { calls.push('delivery'); internal.revoke(); return recordWith('pushed'); },
    async dispatchDelivery() { calls.push('dispatchDelivery'); return recordWith('delivered'); },
    async resolveDelivery() { calls.push('resolveDelivery'); return recordWith('pushed'); },
  };
  await assert.rejects(() => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_SCOPE_CHANGED');
  assert.deepEqual(calls, ['delivery']);
});


test('a mismatched delivery id is invalid input before any recovery write', async () => {
  const h = harness('pushed');
  await assert.rejects(() => h.recover({ runId: 'run-1', deliveryId: 'other-delivery' }), (error: unknown) =>
    error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_INVALID_INPUT');
  assert.deepEqual(h.calls, ['delivery']);
});
