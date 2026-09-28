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
  const { lease, revoke } = mintLease();
  let revoked = false;
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { calls.push('delivery'); return recordWith(state); },
    async dispatchDelivery() { calls.push('dispatchDelivery'); return recordWith('delivered'); },
    async resolveDelivery() { calls.push('resolveDelivery'); return recordWith('pushed'); },
  };
  return {
    calls,
    revoke() { revoke(); revoked = true; },
    recover: createDeliveryRecovery({ remote, lease: () => (revoked ? undefined : lease) }).recover,
  };
}

test('recover routes pushed to the PR-only dispatch half exactly once', async () => {
  const h = harness('pushed');
  const view = await h.recover({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.equal(view.state, 'delivered');
  assert.deepEqual(h.calls, ['delivery', 'dispatchDelivery']);
});

test('recover routes unknown to the remote-facts resolve half', async () => {
  const h = harness('unknown');
  const view = await h.recover({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.deepEqual(h.calls, ['delivery', 'resolveDelivery']);
  assert.equal(view.state, 'pushed');
});

test('recover on delivered is idempotent — zero write requests (re-entry safety)', async () => {
  const h = harness('delivered');
  const view = await h.recover({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.equal(view.state, 'delivered');
  assert.deepEqual(h.calls, ['delivery']);
});

test('prepared/dispatched/failed are not recovery windows — DELIVERY_STATE_CONFLICT', async () => {
  for (const state of ['prepared', 'dispatched', 'failed'] as const) {
    const h = harness(state);
    await assert.rejects(
      () => h.recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
      (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_STATE_CONFLICT' && error.message.includes(state),
    );
    assert.deepEqual(h.calls, ['delivery'], `${state} must not trigger any write`);
  }
});

test('no delivery to recover fails closed with DELIVERY_INVALID_INPUT', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return null; },
    async dispatchDelivery() { throw new Error('must not be called'); },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  const { lease } = mintLease();
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_INVALID_INPUT',
  );
});

test('no lease fails closed with DELIVERY_SCOPE_CHANGED before any network', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { throw new Error('must not be called'); },
    async dispatchDelivery() { throw new Error('must not be called'); },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => undefined }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_SCOPE_CHANGED',
  );
});

test('a lease revoked while the write was in flight drops the late result — DELIVERY_SCOPE_CHANGED', async () => {
  const { lease, revoke } = mintLease();
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return recordWith('pushed'); },
    async dispatchDelivery() { revoke(); return recordWith('delivered'); },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_SCOPE_CHANGED',
  );
});

test('a read rejection after lease revocation reports DELIVERY_SCOPE_CHANGED', async () => {
  const { lease, revoke } = mintLease();
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { revoke(); throw new Error('network down'); },
    async dispatchDelivery() { throw new Error('must not be called'); },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_SCOPE_CHANGED',
  );
});

test('a write rejection after lease revocation reports DELIVERY_SCOPE_CHANGED before translating API conflict', async () => {
  const { lease, revoke } = mintLease();
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return recordWith('pushed'); },
    async dispatchDelivery() {
      revoke();
      throw Object.assign(new Error('state conflict'), { status: 409 });
    },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_SCOPE_CHANGED',
  );
});

test('a 409 state conflict translates to DELIVERY_STATE_CONFLICT (both ApiError shapes)', async () => {
  for (const shape of [
    Object.assign(new Error('api error 409'), { status: 409 }),
    Object.assign(new Error('api error 409'), { status: 409, code: 'code_delivery_state_conflict' }),
    Object.assign(new Error('api error 409'), { status: 409, body: { code: 'code_delivery_state_conflict' } }),
    Object.assign(new Error('api error'), { code: 'code_delivery_state_conflict' }),
    Object.assign(new Error('api error'), { body: { code: 'code_delivery_state_conflict' } }),
  ]) {
    const remote: DeliveryRemote & DeliveryRecoveryRemote = {
      async delivery() { return recordWith('pushed'); },
      async dispatchDelivery() { throw shape; },
      async resolveDelivery() { throw new Error('must not be called'); },
    };
    const { lease } = mintLease();
    await assert.rejects(
      () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
      (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_STATE_CONFLICT',
    );
  }
});

test('any other backend failure translates to DELIVERY_BACKEND', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return recordWith('pushed'); },
    async dispatchDelivery() { return Promise.reject(new Error('network down')); },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  const { lease } = mintLease();
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_BACKEND',
  );
});
