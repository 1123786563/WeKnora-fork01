import test from 'node:test';
import assert from 'node:assert/strict';
// W37 carry-forward (W36 review Important): the mobile app must CONSUME the
// server capability handshake. Unknown schema or upgrade_required verdicts
// stop control commands while the safe login surface and the upgrade
// explanation stay visible.
import {
  serverCapabilitiesRequest,
  createProtocolGate,
  parseWorkbenchWireSnapshot,
} from './protocol-gate.ts';

function responder(payload: unknown, error?: Error) {
  return async (request: unknown) => {
    if (error) throw error;
    return payload;
  };
}

test('handshake reads /api/v1/system/capabilities and a full window allows control commands', async () => {
  const request = serverCapabilitiesRequest();
  assert.equal(request.method, 'GET');
  assert.equal(request.path, '/api/v1/system/capabilities');
  const gate = createProtocolGate(async () => (
    { code: 0, msg: 'success', data: { edition: 'standard', capabilities: {}, protocol_minimum: 2, protocol_maximum: 3 } }
  ));
  const verdict = await gate.handshake();
  assert.equal(verdict.mode, 'full');
  assert.equal(verdict.controlCommandsAllowed, true);
  assert.equal(verdict.safeSurface, 'login');
  assert.equal(gate.latest()?.mode, 'full');
});

test('upgrade_required window stops control commands and keeps the login-upgrade surface', async () => {
  const gate = createProtocolGate(responder({ code: 0, msg: 'success', data: { protocol_minimum: 4, protocol_maximum: 5 } }));
  const verdict = await gate.handshake();
  assert.equal(verdict.mode, 'upgrade_required');
  assert.equal(verdict.controlCommandsAllowed, false);
  assert.equal(verdict.safeSurface, 'login-upgrade');
});

test('server_upgrade_required window (server rollback) also stops control commands', async () => {
  const gate = createProtocolGate(responder({ code: 0, msg: 'success', data: { protocol_minimum: 1, protocol_maximum: 2 } }));
  const verdict = await gate.handshake();
  assert.equal(verdict.mode, 'server_upgrade_required');
  assert.equal(verdict.controlCommandsAllowed, false);
  assert.equal(verdict.safeSurface, 'login-upgrade');
});

test('unknown capability schema never becomes an allow decision', async () => {
  const gate = createProtocolGate(responder({ code: 0, msg: 'success', data: { edition: 'standard', capabilities: {} } }));
  const verdict = await gate.handshake();
  assert.equal(verdict.mode, 'unknown_schema');
  assert.equal(verdict.controlCommandsAllowed, false);
  assert.equal(verdict.safeSurface, 'login-upgrade');
});

test('handshake failures degrade fail-closed to unknown_schema instead of throwing', async () => {
  const gate = createProtocolGate(responder(undefined, new Error('offline')));
  const verdict = await gate.handshake();
  assert.equal(verdict.mode, 'unknown_schema');
  assert.equal(verdict.controlCommandsAllowed, false);
  const rejected = createProtocolGate(responder({ code: 1, msg: 'unauthorized' }));
  assert.equal((await rejected.handshake()).mode, 'unknown_schema');
});

test('wire workbench switches map snake_case to the client mirror (no fail-open)', () => {
  const snapshot = parseWorkbenchWireSnapshot({ read_enabled: false, platform_admission: false, worker_drain: true });
  assert.equal(snapshot.readEnabled, false);
  assert.equal(snapshot.platformAdmission, false);
  assert.equal(snapshot.workerDrain, true);
  // camelCase keys are accepted too, but a snake_case key must never be
  // silently read as "unset" (the W36 review fail-open trap).
  const camel = parseWorkbenchWireSnapshot({ readEnabled: false });
  assert.equal(camel.readEnabled, false);
  assert.deepEqual(parseWorkbenchWireSnapshot('nonsense'), {});
});

test('gate notifies subscribers when the verdict changes', async () => {
  const gate = createProtocolGate(responder({ code: 0, msg: 'success', data: { protocol_minimum: 2, protocol_maximum: 3 } }));
  const events: string[] = [];
  gate.subscribe(() => events.push('changed'));
  await gate.handshake();
  assert.deepEqual(events, ['changed']);
});
