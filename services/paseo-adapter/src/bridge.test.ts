import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { BridgeError, commandHash, commandSignature, decodeStartEnvelope, encodeStartEnvelope, type PaseoPort, type StartCommand, validateStartCommand } from './protocol.ts';
import { startViaBridge } from './bridge.ts';

const command = (patch: Partial<StartCommand> = {}): StartCommand => ({
  commandID: 'c', runID: 'r', attemptID: 'a', targetID: 'n', workspaceRef: 'w',
  prompt: 'hi', provider: 'p', epoch: 1, payloadHash: 'hash-1', expiresAt: Date.now() + 10_000, ...patch,
});
const port = (patch: Partial<PaseoPort> = {}): PaseoPort => ({
  create: async () => ({ id: 'a' }), observe: async () => ({ state: 'running' }), cancel: async () => {}, ...patch,
});
const verifier = { serviceIdentity: 'weknora-execution', signingSecret: 'secret', authorizationVersion: 1 };
const admitted = (c: StartCommand) => ({ serviceIdentity: verifier.serviceIdentity, signature: commandSignature(c, verifier.signingSecret), authorizationVersion: 1, commandHash: commandHash(c), targetID: c.targetID, workspaceRef: c.workspaceRef, workspaceTargetID: c.targetID, epoch: c.epoch, verify: async () => {} });

test('expired command never reaches SDK', async () => {
  let calls = 0;
  const c = command({ expiresAt: 10 });
  await assert.rejects(startViaBridge(c, port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe', 20, { admission: admitted(c), admissionVerifier: verifier }), (e: unknown) => e instanceof BridgeError && e.code === 'COMMAND_EXPIRED');
  assert.equal(calls, 0);
});
test('resolves the opaque workspace and sends one fixed create call', async () => {
  const calls: unknown[] = [];
  const c = command();
  const result = await startViaBridge(c, port({ create: async input => { calls.push(input); return { id: 'a' }; } }), async ref => { assert.equal(ref, 'w'); return '/safe'; }, Date.now(), { admission: admitted(c), admissionVerifier: verifier });
  assert.deepEqual(result, { id: 'a' });
  assert.deepEqual(calls, [{ cwd: '/safe', prompt: 'hi', provider: 'p' }]);
});
test('invalid command and workspace do not reach SDK', async () => {
  let calls = 0;
  const invalid = command({ provider: '' });
  await assert.rejects(startViaBridge(invalid, port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe', Date.now(), { admission: admitted(invalid), admissionVerifier: verifier }), /missing provider/);
  const c = command();
  await assert.rejects(startViaBridge(c, port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '', Date.now(), { admission: admitted(c), admissionVerifier: verifier }), /WORKSPACE_FORBIDDEN/);
  assert.equal(calls, 0);
});
test('timeout and cancellation map to stable bridge errors', async () => {
  const c = command();
  await assert.rejects(startViaBridge(c, port({ create: () => new Promise(() => {}) }), async () => '/safe', Date.now(), { admission: admitted(c), admissionVerifier: verifier, timeoutMs: 5 }), /BRIDGE_TIMEOUT/);
  const controller = new AbortController(); controller.abort();
  const c2 = command();
  await assert.rejects(startViaBridge(c2, port(), async () => '/safe', Date.now(), { admission: admitted(c2), admissionVerifier: verifier, signal: controller.signal }), /BRIDGE_CANCELLED/);
});

test('deadline guard prevents resolver and SDK work before dispatch', async () => {
  let resolverCalls = 0;
  let sdkCalls = 0;
  const c = command();
  await assert.rejects(startViaBridge(c, port({ create: async () => { sdkCalls++; return { id: 'a' }; } }), async () => { resolverCalls++; return '/safe'; }, Date.now(), { admission: admitted(c), admissionVerifier: verifier, timeoutMs: 0 }), /BRIDGE_TIMEOUT/);
  assert.equal(resolverCalls, 0);
  assert.equal(sdkCalls, 0);
});

test('admission and resolver errors fail closed before SDK', async () => {
  let calls = 0;
  const c = command();
  await assert.rejects(startViaBridge(c, port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe', Date.now(), { admission: { ...admitted(c), signature: 'hmac-sha256:tampered' }, admissionVerifier: verifier }), /ADMISSION_FORBIDDEN/);
  const c2 = command();
  await assert.rejects(startViaBridge(c2, port({ create: async () => { calls++; return { id: 'a' }; } }), async () => { throw new Error('not owned'); }, Date.now(), { admission: admitted(c2), admissionVerifier: verifier }), /WORKSPACE_FORBIDDEN/);
  const c3 = command({ commandID: 'x'.repeat(513) });
  await assert.rejects(startViaBridge(c3, port({ create: async () => { calls++; return { id: 'a' }; } }), async () => '/safe', Date.now(), { admission: admitted(c3), admissionVerifier: verifier }), /identifier/);
  assert.equal(calls, 0);
});

test('canonical JSON fixture uses the same envelope and field names as Go', async () => {
  const fixture = JSON.parse(await readFile(new URL('../../../internal/execution/testdata/start-command.json', import.meta.url), 'utf8')) as { version: number; operation: string; payload: StartCommand };
  assert.equal(fixture.version, 1);
  assert.equal(fixture.operation, 'start');
  assert.equal(fixture.payload.commandID, 'cmd-1');
  assert.equal(fixture.payload.workspaceRef, 'workspace-1');
  validateStartCommand(fixture.payload);
  const serialized = JSON.stringify(fixture);
  assert.deepEqual(JSON.parse(serialized), fixture);
  assert.deepEqual(decodeStartEnvelope(encodeStartEnvelope(fixture.payload)), fixture.payload);
  assert.throws(() => decodeStartEnvelope(JSON.stringify({ ...fixture, extra: true })), /INVALID_COMMAND/);
});
