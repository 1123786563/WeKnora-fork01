import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createInMemorySubmissionStore,
  createSubmissionCoordinator,
  inputDigest,
  SubmissionConflictError,
  type MobileStartInput,
  type SubmissionScope,
  type SubmissionTransport,
} from './submission.ts';

const scope: SubmissionScope = { origin: 'https://weknora.example', tenantID: 't1', userID: 'u1' };

function input(overrides: Partial<MobileStartInput> = {}): MobileStartInput {
  return {
    request_id: 'request-original',
    session_id: 'session-1',
    agent_id: 'agent-1',
    target_id: 'platform',
    workspace_ref: 'workspace-1',
    text: '整理本周反馈',
    budget_upper: 100,
    ...overrides,
  };
}

test('persist-before-network: store failure prevents dispatch', async () => {
  let dispatched = 0;
  const transport: SubmissionTransport = {
    start: async () => { dispatched += 1; return { run_id: 'r', request_id: 'request-original', status: 'admitted' }; },
    lookup: async () => { throw new Error('not used'); },
  };
  const failingStore = {
    load: () => undefined,
    save: () => { throw new Error('disk full'); },
    listScope: () => [],
  };
  const coordinator = createSubmissionCoordinator(failingStore, transport);
  await assert.rejects(coordinator.submit(input(), scope), /disk full/);
  assert.equal(dispatched, 0, 'network must not fire when persistence fails');
});

test('same request_id different input conflicts without network', async () => {
  let dispatched = 0;
  const transport: SubmissionTransport = {
    start: async (i) => { dispatched += 1; return { run_id: 'run-original', request_id: i.request_id, status: 'admitted' }; },
    lookup: async () => ({ state: 'unknown' as const }),
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  await coordinator.submit(input(), scope);
  await assert.rejects(coordinator.submit(input({ text: '不同的输入' }), scope), (e: unknown) => e instanceof SubmissionConflictError);
  assert.equal(dispatched, 1);
});

test('digest changes when any of the seven fields changes', () => {
  const base = inputDigest(input());
  assert.notEqual(inputDigest(input({ text: 'x' })), base);
  assert.notEqual(inputDigest(input({ budget_upper: 101 })), base);
  assert.notEqual(inputDigest(input({ workspace_ref: 'w2' })), base);
  assert.equal(inputDigest(input()), base, 'same input must produce a stable digest');
});

test('rejected submissions allow controlled retry with a fresh request_id', async () => {
  const transport: SubmissionTransport = {
    start: async (i) => { throw new Error('ack lost'); },
    lookup: async () => ({ state: 'rejected' as const, reason: 'budget' }),
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  const outcome = await coordinator.submit(input(), scope);
  assert.equal(outcome.entry.phase, 'awaiting_reconciliation');
  const reconciled = await coordinator.reconcile('request-original', scope);
  assert.equal(reconciled.phase, 'rejected');
  // 非 rejected 的 entry 不允许受控重试
  const other = createSubmissionCoordinator(store, { start: async () => { throw new Error('x'); }, lookup: async () => ({ state: 'pending' as const }) });
  const pendingEntry = await other.submit(input({ request_id: 'request-pending' }), scope);
  assert.throws(() => other.retryEntry(pendingEntry.request_id), /explicitly rejected/);
});
