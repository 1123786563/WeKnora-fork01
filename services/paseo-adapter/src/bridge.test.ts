import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { BridgeError, type PaseoPort, type StartCommand, validateStartCommand } from './protocol.ts';
import { startViaBridge } from './bridge.ts';

const command = (patch: Partial<StartCommand> = {}): StartCommand => ({
  commandID: 'c', runID: 'r', attemptID: 'a', targetID: 'n', workspaceRef: 'w',
  prompt: 'hi', provider: 'p', epoch: 1, expiresAt: Date.now() + 10_000, ...patch,
});
const port = (patch: Partial<PaseoPort> = {}): PaseoPort => ({
  create: async () => ({ id: 'a' }), observe: async () => ({ state: 'running' }), cancel: async () => {}, ...patch,
});
const admitted = { admit: async () => {} };

test('expired command never reaches SDK', async () => {
  let calls = 0;
  await assert.rejects(startViaBridge(command({ expiresAt: 10 }), port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe', 20, admitted), (e: unknown) => e instanceof BridgeError && e.code === 'COMMAND_EXPIRED');
  assert.equal(calls, 0);
});
test('resolves the opaque workspace and sends one fixed create call', async () => {
  const calls: unknown[] = [];
  const result = await startViaBridge(command(), port({ create: async input => { calls.push(input); return { id: 'a' }; } }), async ref => { assert.equal(ref, 'w'); return '/safe'; }, Date.now(), admitted);
  assert.deepEqual(result, { id: 'a' });
  assert.deepEqual(calls, [{ cwd: '/safe', prompt: 'hi', provider: 'p' }]);
});
test('invalid command and workspace do not reach SDK', async () => {
  let calls = 0;
  await assert.rejects(startViaBridge(command({ provider: '' }), port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe', Date.now(), admitted), /missing provider/);
  await assert.rejects(startViaBridge(command(), port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '', Date.now(), admitted), /WORKSPACE_FORBIDDEN/);
  assert.equal(calls, 0);
});
test('timeout and cancellation map to stable bridge errors', async () => {
  await assert.rejects(startViaBridge(command(), port({ create: () => new Promise(() => {}) }), async () => '/safe', Date.now(), { ...admitted, timeoutMs: 5 }), /BRIDGE_TIMEOUT/);
  const controller = new AbortController(); controller.abort();
  await assert.rejects(startViaBridge(command(), port(), async () => '/safe', Date.now(), { ...admitted, signal: controller.signal }), /BRIDGE_CANCELLED/);
});

test('admission and resolver errors fail closed before SDK', async () => {
  let calls = 0;
  await assert.rejects(startViaBridge(command(), port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe', Date.now(), { admit: async () => { throw new Error('target mismatch'); } }), /ADMISSION_FORBIDDEN/);
  await assert.rejects(startViaBridge(command(), port({ create: async () => { calls++; return { id: 'a' }; } }), async () => { throw new Error('not owned'); }, Date.now(), admitted), /WORKSPACE_FORBIDDEN/);
  await assert.rejects(startViaBridge(command({ commandID: 'x'.repeat(513) }), port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe', Date.now(), admitted), /identifier/);
  assert.equal(calls, 0);
});

test('canonical JSON fixture uses the same envelope and field names as Go', async () => {
  const fixture = JSON.parse(await readFile(new URL('../../../internal/execution/testdata/start-command.json', import.meta.url), 'utf8')) as { version: number; operation: string; payload: StartCommand };
  assert.equal(fixture.version, 1);
  assert.equal(fixture.operation, 'start');
  assert.equal(fixture.payload.commandID, 'cmd-1');
  assert.equal(fixture.payload.workspaceRef, 'workspace-1');
  validateStartCommand(fixture.payload);
});
