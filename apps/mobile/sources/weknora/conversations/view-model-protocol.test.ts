import test from 'node:test';
import assert from 'node:assert/strict';
// W37 carry-forward (W36 review Important): the product conversation must
// consume the protocol gate — an unknown/upgrade verdict stops cancel/steer
// BEFORE any network call while the safe surface (login, reads, upgrade
// explanation) stays available.
import { createProductConversationViewModel } from './view-model.ts';
import { createProductScope } from '../platform/product-session.ts';
import type { ClientGateVerdict } from '@weknora/domain/mobile';

function verdict(overrides: Partial<ClientGateVerdict> = {}): ClientGateVerdict {
  return { mode: 'full', controlCommandsAllowed: true, safeSurface: 'login', ...overrides };
}

function gateWith(verdictValue?: ClientGateVerdict) {
  const listeners = new Set<() => void>();
  let current = verdictValue;
  return {
    latest: () => current,
    subscribe: (listener: () => void) => { listeners.add(listener); return () => listeners.delete(listener); },
    __set: (next: ClientGateVerdict) => { current = next; listeners.forEach((l) => l()); },
  };
}

function buildModel(gate?: ReturnType<typeof gateWith>) {
  const commands: string[] = [];
  const scope = createProductScope({ origin: 'https://api.example', userId: 'u1', tenantId: 't1' });
  const model = createProductConversationViewModel({
    scope,
    spaceId: 'sp1',
    sessionId: 's1',
    agent: { id: 'a1', name: 'Agent' },
    targetId: 't1',
    workspaceRef: 'w1',
    budgetUpper: 0,
    executions: {
      start: async () => ({ run_id: 'run-1', request_id: 'q1', status: 'queued' }),
      lookup: async () => ({ state: 'unknown' as const }),
      command: async (_runID, input) => { commands.push(input.action); return {}; },
    },
    protocolGate: gate,
  });
  return { model, commands, scope };
}

test('an upgrade_required verdict stops cancel and steer with zero network calls', async () => {
  const { model, commands } = buildModel(gateWith(verdict({ mode: 'upgrade_required', controlCommandsAllowed: false, safeSurface: 'login-upgrade' })));
  await assert.rejects(model.commands.cancel('run-1'), /PROTOCOL_UPGRADE_REQUIRED/);
  await assert.rejects(model.commands.steer('run-1', 'change plan'), /PROTOCOL_UPGRADE_REQUIRED/);
  assert.deepEqual(commands, []);
  assert.equal(model.capabilities.canCancel, false);
  assert.equal(model.capabilities.canSteer, false);
  // The upgrade explanation stays on the safe surface.
  assert.match(model.protocolNotice ?? '', /升级/);
});

test('an unfinished handshake (no verdict yet) is fail-closed: no control commands', async () => {
  const { model, commands } = buildModel(gateWith(undefined));
  await assert.rejects(model.commands.cancel('run-1'), /PROTOCOL_UPGRADE_REQUIRED/);
  await assert.rejects(model.commands.steer('run-1', 'text'), /PROTOCOL_UPGRADE_REQUIRED/);
  assert.deepEqual(commands, []);
});

test('an unknown_schema verdict stops control commands too', async () => {
  const { model, commands } = buildModel(gateWith(verdict({ mode: 'unknown_schema', controlCommandsAllowed: false, safeSurface: 'login-upgrade' })));
  await assert.rejects(model.commands.cancel('run-1'), /PROTOCOL_UPGRADE_REQUIRED/);
  assert.deepEqual(commands, []);
  assert.match(model.protocolNotice ?? '', /兼容/);
});

test('a full verdict passes commands through to the execution API', async () => {
  const { model, commands } = buildModel(gateWith(verdict()));
  await model.commands.cancel('run-1');
  await model.commands.steer('run-1', 'change plan');
  assert.deepEqual(commands, ['cancel', 'steer']);
  assert.equal(model.capabilities.canCancel, true);
  assert.equal(model.protocolNotice, undefined);
});

test('a late handshake reopens control commands and clears the notice', async () => {
  const gate = gateWith(verdict({ mode: 'upgrade_required', controlCommandsAllowed: false, safeSurface: 'login-upgrade' }));
  const { model, commands } = buildModel(gate);
  await assert.rejects(model.commands.cancel('run-1'), /PROTOCOL_UPGRADE_REQUIRED/);
  gate.__set(verdict());
  assert.equal(model.capabilities.canCancel, true);
  assert.equal(model.protocolNotice, undefined);
  await model.commands.steer('run-1', 'ok now');
  assert.deepEqual(commands, ['steer']);
});

test('without a gate the legacy behaviour is unchanged', async () => {
  const { model, commands } = buildModel(undefined);
  assert.equal(model.capabilities.canCancel, true);
  assert.equal(model.capabilities.canSteer, true);
  assert.equal(model.protocolNotice, undefined);
  await model.commands.cancel('run-1');
  assert.deepEqual(commands, ['cancel']);
});
