import assert from 'node:assert/strict';
import test from 'node:test';
import { createConversationViewModel, createSendController } from './view-model.ts';

test('failed send keeps the draft and releases the busy lock', async () => {
  const vm = createSendController(async () => { throw new Error('offline'); });
  await assert.rejects(vm.submit('review my report', 'q1'), /offline/);
  assert.equal(vm.draft(), 'review my report');
  assert.equal(vm.busy(), false);
});

test('successful send clears the draft and concurrent sends are rejected', async () => {
  let resolve!: () => void;
  const sent = new Promise<void>((done) => { resolve = done; });
  const calls: Array<[string, string]> = [];
  const vm = createSendController(async (text, requestID) => {
    calls.push([text, requestID]);
    await sent;
  });
  const first = vm.submit('hello', 'request-1');
  assert.equal(vm.busy(), true);
  await assert.rejects(vm.submit('second', 'request-2'), /SEND_IN_PROGRESS/);
  resolve();
  await first;
  assert.deepEqual(calls, [['hello', 'request-1']]);
  assert.equal(vm.draft(), '');
  assert.equal(vm.busy(), false);
});

test('view model maps execution and keeps pending/unknown states visible', () => {
  const model = createConversationViewModel({
    scope: { origin: 'https://api.example', userId: 'u1', tenantId: 't1', spaceId: 's1' },
    messages: [{ id: 'm1', role: 'user', text: 'hello', agentID: 'agent-1' }],
    pendingInteractions: [{ id: 'p1', kind: 'approval', status: 'pending', label: 'Approve' }],
    capabilities: { canCancel: true, canSteer: false },
    execution: { run_id: 'run-1', request_id: 'q1', status: 'unknown' },
    commands: { cancel: async () => undefined, steer: async () => undefined },
  });
  assert.deepEqual(model.scope, { origin: 'https://api.example', userId: 'u1', tenantId: 't1', spaceId: 's1' });
  assert.equal(model.messages[0]?.id, 'm1');
  assert.equal(model.pendingInteractions[0]?.status, 'pending');
  assert.equal(model.execution?.status, 'unknown');
  assert.equal(model.capabilities.canSteer, false);
  assert.equal(typeof model.commands.cancel, 'function');
});
