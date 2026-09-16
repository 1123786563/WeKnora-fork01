import test from 'node:test';
import assert from 'node:assert/strict';
import { BridgeError, type PaseoPort, type StartCommand } from './protocol.ts';
import { startViaBridge } from './bridge.ts';

const command = (patch: Partial<StartCommand> = {}): StartCommand => ({
  commandID: 'c', runID: 'r', attemptID: 'a', targetID: 'n', workspaceRef: 'w',
  prompt: 'hi', provider: 'p', epoch: 1, expiresAt: Date.now() + 10_000, ...patch,
});
const port = (patch: Partial<PaseoPort> = {}): PaseoPort => ({
  create: async () => ({ id: 'a' }), observe: async () => ({ state: 'running' }), cancel: async () => {}, ...patch,
});

test('expired command never reaches SDK', async () => {
  let calls = 0;
  await assert.rejects(startViaBridge(command({ expiresAt: 10 }), port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe', 20), (e: unknown) => e instanceof BridgeError && e.code === 'COMMAND_EXPIRED');
  assert.equal(calls, 0);
});
test('resolves the opaque workspace and sends one fixed create call', async () => {
  const calls: unknown[] = [];
  const result = await startViaBridge(command(), port({ create: async input => { calls.push(input); return { id: 'a' }; } }), async ref => { assert.equal(ref, 'w'); return '/safe'; }, Date.now());
  assert.deepEqual(result, { id: 'a' });
  assert.deepEqual(calls, [{ cwd: '/safe', prompt: 'hi', provider: 'p' }]);
});
test('invalid command and workspace do not reach SDK', async () => {
  let calls = 0;
  await assert.rejects(startViaBridge(command({ provider: '' }), port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe'), /missing provider/);
  await assert.rejects(startViaBridge(command(), port({ create: async () => { calls++; return { id: 'a' }; } }), async () => ''), /WORKSPACE_FORBIDDEN/);
  assert.equal(calls, 0);
});
test('timeout and cancellation map to stable bridge errors', async () => {
  await assert.rejects(startViaBridge(command(), port({ create: () => new Promise(() => {}) }), async () => '/safe', Date.now(), { timeoutMs: 5 }), /BRIDGE_TIMEOUT/);
  const controller = new AbortController(); controller.abort();
  await assert.rejects(startViaBridge(command(), port(), async () => '/safe', Date.now(), { signal: controller.signal }), /BRIDGE_CANCELLED/);
});
