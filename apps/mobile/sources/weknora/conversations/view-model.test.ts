import assert from 'node:assert/strict';
import test from 'node:test';
import { createProductAuthSession } from '@weknora/api-client';
import type { ExecutionEvent } from '@weknora/contracts';
import { createConversationViewModel, createProductConversationViewModel, createProductExecutionApi, createSendController } from './view-model.ts';

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

test('product VM consumes W09 online events from the projection watermark', async () => {
  const requests: Array<{ runID: string; cursor: string | undefined }> = [];
  let emit!: (event: ExecutionEvent) => void;
  const executions = {
    start: async () => ({ run_id: 'run-1', request_id: 'r1', status: 'running' }),
    lookup: async () => ({ state: 'admitted' as const, run_id: 'run-1' }),
    command: async () => ({}),
    snapshot: async (_runID: string) => ({ execution: { schema_version: 1 as const, run_id: 'run-1', session_id: 's1', revision: 1, driver: 'platform' as const, run_status: 'running' as const, execution_status: 'running', settlement_status: 'reserved', seq: 1, capabilities: {} }, watermark: 1, events: [{ schema_version: 1 as const, run_id: 'run-1', attempt_id: 'a1', seq: 1, type: 'message.created', occurred_at: '2026-09-16T00:00:00Z', payload: { message: { id: 'm1', blocks: [{ id: 'b1', kind: 'text', text: 'hi' }] } } }] }),
    stream: async (runID: string, cursor: string | undefined, onEvent: (event: ExecutionEvent) => void) => { requests.push({ runID, cursor }); emit = onEvent; },
  };
  const scope = { identity: () => ({ origin: 'https://api.example', userId: 'u1', tenantId: 't1' }), capture: () => ({ generation: 1, signal: new AbortController().signal }), accept: () => true } as any;
  const model = createProductConversationViewModel({ scope, spaceId: 's1', sessionId: 's1', agent: { id: 'a1', name: 'Agent' }, targetId: 't1', workspaceRef: 'w1', budgetUpper: 1, executions, projection: { load: async (runID) => {
    const snapshot = await executions.snapshot(runID);
    const { projectExecutionSnapshot } = await import('./execution-projection.ts');
    return projectExecutionSnapshot(snapshot);
  } } });
  await model.send!.submit('hello', 'r1');
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(requests, [{ runID: 'run-1', cursor: '1' }]);
  emit({ schema_version: 1, run_id: 'run-1', attempt_id: 'a1', seq: 2, type: 'text.delta', occurred_at: '2026-09-16T00:00:01Z', payload: { message_id: 'm1', delta: ' there' } });
  assert.equal(model.messages.length, 1);
  assert.equal(model.messages[0]?.text, 'hi there');
});

test('commits online events before projecting them when durable storage is provided', async () => {
  const committed: number[] = [];
  let emit!: (event: ExecutionEvent) => void;
  const executions = {
    start: async () => ({ run_id: 'run-1', request_id: 'r1', status: 'running' }),
    lookup: async () => ({ state: 'admitted' as const, run_id: 'run-1' }),
    command: async () => ({}),
    snapshot: async () => ({ execution: { schema_version: 1 as const, run_id: 'run-1', session_id: 's1', revision: 1, driver: 'platform' as const, run_status: 'running' as const, execution_status: 'running', settlement_status: 'reserved', seq: 0, capabilities: {} }, watermark: 0, events: [] }),
    stream: async (_runID: string, _cursor: string | undefined, onEvent: (event: ExecutionEvent) => void) => { emit = onEvent; },
  };
  const scope = { identity: () => ({ origin: 'https://api.example', userId: 'u1', tenantId: 't1' }), capture: () => ({ generation: 1, signal: new AbortController().signal }), accept: () => true } as any;
  const model = createProductConversationViewModel({ scope, spaceId: 's1', sessionId: 's1', agent: { id: 'a1', name: 'Agent' }, targetId: 't1', workspaceRef: 'w1', budgetUpper: 1, executions, projection: { load: async () => ({ messages: [], pendingInteractions: [], execution: { runID: 'run-1', requestID: 'r1', status: 'running' }, watermark: 0 }) }, eventStorage: { read: async () => [], commit: async (event) => { committed.push(event.seq); } } });
  await model.send!.submit('hello', 'r1');
  await new Promise((resolve) => setTimeout(resolve, 0));
  emit({ schema_version: 1, run_id: 'run-1', attempt_id: 'a1', seq: 1, type: 'execution.succeeded', occurred_at: '2026-09-16T00:00:01Z', payload: { status: 'succeeded' } });
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(committed, [1]);
  assert.equal(model.execution?.status, 'succeeded');
});

test('cancel and steer fence late responses after a scope switch', async () => {
  let generation = 1;
  let resolve!: () => void;
  const calls: AbortSignal[] = [];
  const executions = {
    start: async () => ({ run_id: 'run-1', request_id: 'r1', status: 'pending' }),
    lookup: async () => ({ state: 'pending' as const }),
    command: async (_run: string, _input: unknown, signal?: AbortSignal) => { calls.push(signal!); await new Promise<void>((done) => { resolve = done; }); },
  };
  const scope = { identity: () => ({ origin: 'https://api.example', userId: 'u1', tenantId: 't1' }), capture: () => ({ generation, signal: new AbortController().signal }), accept: (value: number) => value === generation } as any;
  const model = createProductConversationViewModel({ scope, spaceId: 's1', sessionId: 's1', agent: { id: 'a1', name: 'Agent' }, targetId: 't1', workspaceRef: 'w1', budgetUpper: 1, executions });
  const pending = model.commands.cancel('run-1', 1);
  generation = 2;
  resolve();
  await assert.rejects(pending, /SCOPE_CHANGED/);
  assert.equal(calls.length, 1);
});

test('product command and approval controls use the real authenticated API adapter', async () => {
  const requests: any[] = [];
  const credentials = { read: async () => ({ kind: 'bearer' as const, accessToken: 'access' }), write: async () => undefined, clear: async () => undefined };
  const authSession = createProductAuthSession({ baseURL: 'https://api.example', credentials, transport: { send: async (request: any) => {
    requests.push(request);
    if (request.url.includes('/tool-approvals/')) return { status: 200, headers: {}, body: { success: true } };
    return { status: 200, headers: {}, body: { success: true, data: { run_id: 'run-1', action: request.body.action } } };
  } } });
  const api = createProductExecutionApi({
    origin: 'https://api.example',
    credential: { kind: 'bearer', accessToken: 'access' },
    scope: { capture: () => ({ generation: 1, signal: new AbortController().signal }), accept: () => true } as any,
    authSession,
  });
  await api.command('run-1', { action: 'cancel', expected_revision: 4 });
  await api.command('run-1', { action: 'steer', text: 'continue', expected_revision: 5 });
  await api.decide!('p-1', { action: 'approve', expected_revision: 6 });
  assert.deepEqual(requests.map((request) => ({ path: new URL(request.url).pathname, body: request.body, authorization: request.headers.authorization })), [
    { path: '/api/v1/workbench/executions/run-1/commands', body: { action: 'cancel', expected_revision: 4 }, authorization: 'Bearer access' },
    { path: '/api/v1/workbench/executions/run-1/commands', body: { action: 'steer', text: 'continue', expected_revision: 5 }, authorization: 'Bearer access' },
    { path: '/api/v1/agent/tool-approvals/p-1', body: { decision: 'approve', expected_revision: 6 }, authorization: 'Bearer access' },
  ]);
});
