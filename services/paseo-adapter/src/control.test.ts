import test from 'node:test';
import assert from 'node:assert/strict';
import { BridgeError } from './protocol.ts';
import { cancelAndObserve, encodeControlCommand, type ControlPort } from './control.ts';

const cancel = { action: 'cancel' as const, commandID: 'c1', runID: 'r1', externalID: 'e1', epoch: 2 };
test('control envelope is canonical and rejects unknown or incomplete fields', () => {
  assert.deepEqual(JSON.parse(encodeControlCommand(cancel)), { version: 1, operation: 'control', payload: cancel });
  assert.throws(() => encodeControlCommand({ ...cancel, externalID: '' }), (e: unknown) => e instanceof BridgeError && e.code === 'INVALID_COMMAND');
  assert.throws(() => encodeControlCommand({ ...cancel, foo: 'x' } as never), /INVALID_COMMAND/);
});
test('cancel is accepted separately from confirmed remote exit', async () => {
  let commands = 0; let observations = 0;
  const port: ControlPort = { command: async () => { commands++; return { accepted: true }; }, observe: async () => { observations++; return observations < 2 ? { processState: 'running', fresh: true, epoch: 2 } : { processState: 'exited', fresh: true, epoch: 2 }; } };
  const result = await cancelAndObserve(port, cancel, 500);
  assert.equal(result.state, 'confirmed'); assert.equal(commands, 1); assert.equal(observations, 2);
});
test('unknown observation keeps the workspace fenced and does not retry command', async () => {
  let commands = 0;
  const port: ControlPort = { command: async () => { commands++; return { accepted: true }; }, observe: async () => ({ processState: 'unknown', fresh: true, epoch: 2 }) };
  const result = await cancelAndObserve(port, cancel, 500);
  assert.equal(result.state, 'unknown'); assert.equal(commands, 1);
});
test('approval command carries pending id, args hash, credential version and revision', () => {
  const encoded = encodeControlCommand({ action: 'submitInteraction', commandID: 'c1', runID: 'r1', externalPendingID: 'p1', actionValue: 'approve', argsHash: 'a'.repeat(64), credentialVersion: 3, expectedRevision: 4 });
  assert.match(encoded, /credentialVersion/); assert.match(encoded, /externalPendingID/); assert.match(encoded, /expectedRevision/);
});
