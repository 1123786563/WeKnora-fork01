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
  for (const field of ['request_id', 'session_id', 'agent_id', 'target_id', 'workspace_ref', 'text', 'budget_upper'] as const) {
    const mutated = field === 'budget_upper' ? input({ budget_upper: 101 }) : input({ [field]: `changed-${field}` } as Partial<MobileStartInput>);
    assert.notEqual(inputDigest(mutated), base, `digest must change when ${field} changes`);
  }
  assert.equal(inputDigest(input()), base, 'same input must produce a stable digest');
});

test('unknown lookup keeps reconciliation state and never re-dispatches', async () => {
  let startCount = 0;
  let lookupCount = 0;
  const transport: SubmissionTransport = {
    start: async (i) => { startCount += 1; throw new Error('ack lost'); },
    lookup: async () => { lookupCount += 1; return { state: 'unknown' }; },
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  const first = await coordinator.submit(input(), scope);
  assert.equal(first.entry.phase, 'awaiting_reconciliation');
  // 重启后重复 submit：走 existing 分支（零 start 调用），unknown 维持对账态
  const restarted = createSubmissionCoordinator(store, transport);
  const second = await restarted.submit(input(), scope);
  assert.equal(second.dispatched, false);
  assert.equal(second.entry.phase, 'awaiting_reconciliation');
  assert.equal(startCount, 1);
  assert.ok(lookupCount >= 1, 'reconciliation must have consulted lookup');
  // pending 同理不重发
  const pendingTransport: SubmissionTransport = {
    start: transport.start,
    lookup: async () => ({ state: 'pending' as const }),
  };
  const third = await createSubmissionCoordinator(store, pendingTransport).submit(input(), scope);
  assert.equal(third.dispatched, false);
  assert.equal(startCount, 1);
});

test('rejected submissions allow controlled retry with a fresh request_id', async () => {
  const transport: SubmissionTransport = {
    start: async (i) => { throw new Error('ack lost'); },
    lookup: async () => ({ state: 'rejected' as const, reason: 'budget' }),
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  // 无形态网络失败后的复查 lookup 已反映 rejected：直接落终态（新语义），
  // 不再恒定 awaiting_reconciliation。
  const outcome = await coordinator.submit(input(), scope);
  assert.equal(outcome.entry.phase, 'rejected');
  const reconciled = await coordinator.reconcile('request-original', scope);
  assert.equal(reconciled.phase, 'rejected');
  // 非 rejected 的 entry 不允许受控重试
  const other = createSubmissionCoordinator(store, { start: async () => { throw new Error('x'); }, lookup: async () => ({ state: 'pending' as const }) });
  const pendingEntry = await other.submit(input({ request_id: 'request-pending' }), scope);
  assert.throws(() => other.retryEntry(pendingEntry.request_id), /explicitly rejected/);
});

test('resume resubmits the same request id only after an explicit unknown lookup', async () => {
  const starts: MobileStartInput[] = [];
  const lookups: string[] = [];
  let failFirstStart = true;
  const transport: SubmissionTransport = {
    start: async (input) => {
      starts.push(input);
      if (failFirstStart) {
        failFirstStart = false;
        throw new Error('request lost');
      }
      return { run_id: 'run-resumed', request_id: input.request_id, status: 'admitted' };
    },
    lookup: async (requestId) => { lookups.push(requestId); return { state: 'unknown' }; },
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  // 首次提交：网络失败 → entry 落在 awaiting_reconciliation，同 ID 保留
  const first = await coordinator.submit(input(), scope);
  assert.equal(first.entry.phase, 'awaiting_reconciliation');
  assert.equal(starts.length, 1);
  // 同一意图重入：先 lookup；unknown → 同 ID 重发（不换 ID）
  const resumed = await coordinator.resume(input(), scope);
  assert.equal(resumed.entry.phase, 'bound');
  assert.equal(resumed.entry.run_id, 'run-resumed');
  assert.equal(resumed.dispatched, true);
  assert.equal(starts.length, 2, 'exactly one resubmission');
  assert.equal(starts[1]!.request_id, input().request_id, 'the SAME request id is reused');
  // 两次 lookup 都是同一 request_id：失败 dispatch 后的复查 + resume 的门控查询。
  assert.deepEqual(lookups, [input().request_id, input().request_id]);
  // 再次重入（已 bound）：直接返回，零网络
  const again = await coordinator.resume(input(), scope);
  assert.equal(again.entry.run_id, 'run-resumed');
  assert.equal(again.dispatched, false);
  assert.equal(starts.length, 2);
  assert.equal(lookups.length, 2);
});

test('resume reconciles instead of resubmitting when the server knows the request', async () => {
  const starts: MobileStartInput[] = [];
  const transport: SubmissionTransport = {
    start: async (input) => { starts.push(input); throw new Error('request lost'); },
    lookup: async () => ({ state: 'admitted' as const, run_id: 'run-original' }),
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  await coordinator.submit(input(), scope);
  const resumed = await coordinator.resume(input(), scope);
  assert.equal(resumed.entry.phase, 'bound');
  assert.equal(resumed.entry.run_id, 'run-original');
  assert.equal(resumed.dispatched, false, 'a server-known request is reconciled, never re-POSTed');
  assert.equal(starts.length, 1);
});

test('resume rejects an entry persisted under a different scope with zero network', async () => {
  let network = 0;
  const transport: SubmissionTransport = {
    start: async () => { network += 1; throw new Error('unused'); },
    lookup: async () => { network += 1; throw new Error('unused'); },
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  await coordinator.submit(input(), scope);
  await assert.rejects(
    coordinator.resume(input(), { origin: 'https://other.example', tenantID: 't1', userID: 'u1' }),
    SubmissionConflictError,
  );
  assert.equal(network, 2, 'the original submit used one start + one follow-up lookup; the scope-conflicting resume added zero network');
});

test('resume with a changed input digest conflicts without network', async () => {
  let network = 0;
  const transport: SubmissionTransport = {
    start: async () => { network += 1; throw new Error('unused'); },
    lookup: async () => { network += 1; throw new Error('unused'); },
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  await coordinator.submit(input(), scope);
  await assert.rejects(coordinator.resume(input({ text: '不同的目标' }), scope), SubmissionConflictError);
  assert.equal(network, 2, 'the original submit used one start + one follow-up lookup; the digest-conflicting resume added zero network');
});

test('a deterministic 4xx dispatch failure lands rejected (the intent can reach a terminal state)', async () => {
  const store = createInMemorySubmissionStore();
  const transport: SubmissionTransport = {
    start: async () => { throw Object.assign(new Error('budget ceiling exceeded'), { status: 422 }); },
    lookup: async () => ({ state: 'unknown' as const }),
  };
  const coordinator = createSubmissionCoordinator(store, transport);
  const outcome = await coordinator.submit(input(), scope);
  assert.equal(outcome.entry.phase, 'rejected', '确定性 4xx 拒绝必须可达 rejected 终态（B3-F44）——不得永远 awaiting_reconciliation');
  // 受控重入（resume）读取同一终态：上层得以停止同 ID 重试引导
  const resumed = await coordinator.resume(input(), scope);
  assert.equal(resumed.entry.phase, 'rejected');
  assert.equal(resumed.dispatched, false);
});

test('a lost 4xx response settles rejected via the follow-up lookup, a transport failure stays reconciling', async () => {
  const store = createInMemorySubmissionStore();
  let lookupState: 'unknown' | 'rejected' = 'unknown';
  const transport: SubmissionTransport = {
    start: async () => { throw new Error('response lost'); }, // 网络/响应丢失（无 status 形态）
    lookup: async () => ({ state: lookupState }),
  };
  const coordinator = createSubmissionCoordinator(store, transport);
  const first = await coordinator.resume(input(), scope);
  assert.equal(first.entry.phase, 'awaiting_reconciliation', '无形态网络失败保持对账语义（既有行为不回归）');
  lookupState = 'rejected'; // 服务端其实已处理为拒绝
  const second = await coordinator.resume(input(), scope);
  assert.equal(second.entry.phase, 'rejected', 'lookup 反映 rejected 时经对账落终态');
  assert.equal(second.dispatched, false);
});

test('resume reuses its lookup result instead of looking it up twice', async () => {
  const store = createInMemorySubmissionStore();
  let lookups = 0;
  const transport: SubmissionTransport = {
    start: async () => { throw new Error('not used'); },
    lookup: async () => { lookups += 1; return { state: 'pending' as const }; },
  };
  const coordinator = createSubmissionCoordinator(store, transport);
  await coordinator.submit(input(), scope); // 首次提交落 awaiting（start 抛错路径复查 lookup）
  const before = lookups;
  await coordinator.resume(input(), scope); // resume 的非 unknown 分支：lookup 1 次（不再经 reconcile 双查）
  assert.equal(lookups, before + 1, 'B3-F62：resume 不得对同一 request_id 连查两次');
});
