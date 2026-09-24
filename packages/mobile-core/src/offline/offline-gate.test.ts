import test from 'node:test';
import assert from 'node:assert/strict';
import { createOfflineGate, OfflineGateError, type NetworkStatusPort, type OfflineActionKind } from './offline-gate.ts';

const KINDS: OfflineActionKind[] = ['run', 'approval', 'budget', 'external-action'];

test('an offline status blocks every dangerous action kind before any dispatch', async () => {
  const gate = createOfflineGate({ online: async () => false });
  assert.equal(await gate.status(), 'offline');
  for (const kind of KINDS) {
    await assert.rejects(gate.assertOnline(kind), (error: unknown) => error instanceof OfflineGateError && error.code === 'OFFLINE_ACTION_BLOCKED' && error.action === kind);
  }
});

test('an online status passes every dangerous action kind', async () => {
  const gate = createOfflineGate({ online: async () => true });
  assert.equal(await gate.status(), 'online');
  for (const kind of KINDS) await gate.assertOnline(kind);
});

test('a failing network probe fails closed as offline', async () => {
  const gate = createOfflineGate({ online: async () => { throw new Error('probe timeout'); } });
  assert.equal(await gate.status(), 'offline');
  await assert.rejects(gate.assertOnline('run'), (error: unknown) => error instanceof OfflineGateError);
});

test('an absent status channel passes through: physical offline still fails at the transport seam', async () => {
  const gate = createOfflineGate(undefined);
  assert.equal(await gate.status(), 'online');
  for (const kind of KINDS) await gate.assertOnline(kind);
});
