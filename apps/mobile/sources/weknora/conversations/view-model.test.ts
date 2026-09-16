import assert from 'node:assert/strict';
import test from 'node:test';
import { createConversationViewModel, createProductConversationViewModel, createSendController } from './view-model.ts';

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

test('product VM uses a fresh request id and refreshes unknown admission', async () => {
  const calls: string[] = [];
  const executions = {
    start: async (input: { request_id: string }) => { calls.push(`start:${input.request_id}`); return { run_id: 'run-1', request_id: input.request_id, status: 'unknown' }; },
    lookup: async (requestID: string) => { calls.push(`lookup:${requestID}`); return { state: 'unknown' as const, reason: 'timeout' }; },
    command: async () => ({}),
  };
  const scope = { identity: () => ({ origin: 'https://api.example', userId: 'u1', tenantId: 't1' }), capture: () => ({ generation: 1, signal: new AbortController().signal }), accept: () => true } as any;
  const model = createProductConversationViewModel({ scope, spaceId: 's1', sessionId: 's1', agent: { id: 'a1', name: 'Agent' }, targetId: 't1', workspaceRef: 'w1', budgetUpper: 1, executions });
  await model.send!.submit('hello', 'request-unique');
  assert.equal(model.execution?.requestID, 'request-unique');
  assert.equal(model.execution?.status, 'unknown');
  assert.deepEqual(calls, ['start:request-unique', 'lookup:request-unique']);
});

test('product VM restores a persisted pending request and rechecks it after remount', async () => {
  const calls: string[] = [];
  let persisted: { requestID: string; runID?: string; status: string } | undefined = { requestID: 'q-restart', runID: 'run-old', status: 'pending' };
  const executions = {
    start: async () => ({ run_id: 'run-1', request_id: 'q', status: 'pending' }),
    lookup: async (requestID: string) => { calls.push(requestID); return { state: 'admitted' as const, run_id: 'run-new' }; },
    command: async () => ({}),
  };
  const scope = { identity: () => ({ origin: 'https://api.example', userId: 'u1', tenantId: 't1' }), capture: () => ({ generation: 1, signal: new AbortController().signal }), accept: () => true } as any;
  const storage = { getLatest: async () => persisted, set: async (record: typeof persisted) => { persisted = record; } };
  const model = createProductConversationViewModel({ scope, spaceId: 's1', sessionId: 's1', agent: { id: 'a1', name: 'Agent' }, targetId: 't1', workspaceRef: 'w1', budgetUpper: 1, executions, requestStorage: storage });
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(model.execution?.requestID, 'q-restart');
  assert.equal(model.execution?.runID, 'run-new');
  assert.deepEqual(calls, ['q-restart']);
});

test('product VM approvals call typed decision and update the card state', async () => {
  const decisions: Array<[string, string, number]> = [];
  const executions = {
    start: async () => ({ run_id: 'run-1', request_id: 'r1', status: 'pending' }),
    lookup: async () => ({ state: 'pending' as const }),
    command: async () => ({}),
    decide: async (id: string, input: { action: 'approve' | 'reject'; expected_revision: number }) => { decisions.push([id, input.action, input.expected_revision]); },
  };
  const scope = { identity: () => ({ origin: 'https://api.example', userId: 'u1', tenantId: 't1' }), capture: () => ({ generation: 1, signal: new AbortController().signal }), accept: () => true } as any;
  const model = createProductConversationViewModel({ scope, spaceId: 's1', sessionId: 's1', agent: { id: 'a1', name: 'Agent' }, targetId: 't1', workspaceRef: 'w1', budgetUpper: 1, executions, pendingInteractions: [{ id: 'interaction-1', kind: 'approval', status: 'pending', label: 'Run tool' }] });
  await model.commands.approve!('interaction-1', 3);
  assert.deepEqual(decisions, [['interaction-1', 'approve', 3]]);
  assert.equal(model.pendingInteractions[0]?.status, 'approved');
});
