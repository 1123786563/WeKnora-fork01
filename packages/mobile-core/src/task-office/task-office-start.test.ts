import test from 'node:test';
import assert from 'node:assert/strict';
import { createInMemorySubmissionStore, type SubmissionStore } from '@weknora/domain/mobile';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { createOfflineGate, OfflineGateError } from '../offline/offline-gate.ts';
import { guardTaskBackend } from '../offline/guarded-ports.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';
import { createInMemoryIntentLog, createTaskOffice, TaskOfficeError, type SubmissionIntentLog, type TaskBackendStartInput } from './task-office.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

const goal = { text: '整理本周反馈并生成周报', agentId: 'agent-1', budgetUpper: 200 };
const scope = { origin: 'https://weknora.example.test', tenantID: 'tenant-1', userID: 'user-1' };

function startOffice(handlers: Parameters<typeof createScenarioTaskBackend>[0], options: { ids?: string[]; store?: SubmissionStore; log?: SubmissionIntentLog } = {}) {
  const backend = createScenarioTaskBackend(handlers);
  const { lease } = leased();
  const office = createTaskOffice({
    backend,
    lease: () => lease,
    ...(options.store === undefined ? {} : { submissionStore: options.store }),
    ...(options.log === undefined ? {} : { intentLog: options.log }),
    ...(options.ids === undefined ? {} : { newRequestId: (() => { const queue = [...options.ids!]; return () => queue.shift() ?? 'req-fallback'; })() }),
  });
  return { backend, office };
}

const startPosts = (backend: ReturnType<typeof createScenarioTaskBackend>): TaskBackendStartInput[] =>
  backend.calls.flatMap((call) => call.kind === 'start' ? [call.input] : []);

test('a failing in-process store prevents the start POST (the session may already exist)', async () => {
  const { backend, office } = startOffice({}, {
    store: { load: () => undefined, save: () => { throw new Error('disk full'); }, listScope: () => [] },
  });
  await assert.rejects(office.start(goal), /disk full/);
  assert.equal(startPosts(backend).length, 0, 'MX-006: store save 失败不得发送 Start');
  // createSession 允许发生在意图落盘之前：它是无 Task/预算副作用的前置网络调用，
  // 与小程序 AgentPage 的 sessions.create → startTask 先例同序（features/home/pages.tsx:26-28）
  assert.equal(backend.calls.filter((call) => call.kind === 'createSession').length, 1);
});

test('an intent log failure prevents the start POST after the session exists', async () => {
  const { backend, office } = startOffice({}, {
    log: {
      save: async () => { throw new Error('intent log disk full'); },
      load: async () => undefined,
      listScope: async () => [],
    },
  });
  await assert.rejects(office.start(goal), /intent log disk full/);
  assert.equal(startPosts(backend).length, 0, 'the durable intent record must precede the start POST');
  assert.equal(backend.calls.filter((call) => call.kind === 'createSession').length, 1);
});

test('a lost acknowledgement reconciles onto the original request and run (no second task)', async () => {
  const posts: string[] = [];
  const { office } = startOffice({
    start: async (input) => { posts.push(input.request_id); if (posts.length === 1) throw new Error('request lost'); return { run_id: 'run-original', request_id: input.request_id, status: 'queued' }; },
    lookup: async () => ({ state: 'admitted', run_id: 'run-original' }),
  }, { ids: ['req-1'] });
  const first = await office.start(goal);
  assert.equal(first.phase, 'awaiting_reconciliation');
  assert.equal(first.requestId, 'req-1');
  assert.equal(posts.length, 1);
  // 用户以同一 request_id 重入（重复点击）：复用原 session，对账命中原 run，零新 POST
  const second = await office.start(goal, { requestId: 'req-1' });
  assert.equal(second.phase, 'bound');
  assert.equal(second.runId, 'run-original');
  assert.equal(second.dispatched, false);
  assert.equal(posts.length, 1, 'the same intent never creates a second task');
});

test('a repeated start with the same request id never dispatches a second POST while unresolved', async () => {
  const posts: string[] = [];
  const { office } = startOffice({
    start: async (input) => { posts.push(input.request_id); throw new Error('offline'); },
    lookup: async () => ({ state: 'pending' }),
  }, { ids: ['req-1'] });
  const first = await office.start(goal);
  assert.equal(first.phase, 'awaiting_reconciliation');
  const second = await office.start(goal, { requestId: first.requestId });
  assert.equal(second.phase, 'awaiting_reconciliation');
  assert.equal(second.dispatched, false, 'pending on the server means: wait, never re-POST');
  assert.equal(posts.length, 1);
});

test('an unknown lookup is the only path that resubmits, and it reuses the same request id AND session', async () => {
  const posts: TaskBackendStartInput[] = [];
  let lookupState: 'unknown' | 'admitted' = 'admitted';
  const { office } = startOffice({
    start: async (input) => { posts.push(input); if (posts.length === 1) throw new Error('request lost'); return { run_id: 'run-9', request_id: input.request_id, status: 'queued' }; },
    lookup: async () => ({ state: lookupState, ...(lookupState === 'admitted' ? { run_id: 'run-9' } : {}) }),
  }, { ids: ['req-1'] });
  await office.start(goal);
  lookupState = 'unknown';
  const resumed = await office.start(goal, { requestId: 'req-1' });
  assert.equal(resumed.phase, 'bound');
  assert.equal(resumed.runId, 'run-9');
  assert.equal(resumed.dispatched, true);
  assert.equal(posts.length, 2);
  assert.equal(posts[1]!.request_id, posts[0]!.request_id, 'the SAME request id is reused (D5)');
  assert.equal(posts[1]!.session_id, posts[0]!.session_id, 'the SAME session is reused — a fresh session would change the digest and fake a new intent');
});

test('attachments that are not ready block submission with zero backend calls', async () => {
  const { backend, office } = startOffice({});
  await assert.rejects(
    office.start({ ...goal, attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }] }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_ATTACHMENTS_NOT_READY',
  );
  assert.equal(backend.calls.length, 0, 'not-ready attachments must not even create a session');
});

test('invalid text, agent or budget surface as TASK_OFFICE_INVALID_INPUT', async () => {
  const { backend, office } = startOffice({});
  for (const bad of [{ ...goal, text: '   ' }, { ...goal, agentId: '' }, { ...goal, budgetUpper: -1 }, { ...goal, budgetUpper: 1.5 }]) {
    await assert.rejects(office.start(bad), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  }
  assert.equal(backend.calls.length, 0);
});

test('the same request id with a different input conflicts with zero network', async () => {
  let posts = 0;
  const { office } = startOffice({
    start: async (input) => { posts += 1; return { run_id: 'r-1', request_id: input.request_id, status: 'queued' }; },
  }, { ids: ['req-1'] });
  await office.start(goal);
  await assert.rejects(
    office.start({ ...goal, text: '另一个不同的目标' }, { requestId: 'req-1' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUBMISSION_CONFLICT',
  );
  assert.equal(posts, 1, 'a conflicting replay performs no second POST');
});

test('a revoked scope rejects start before any persistence or network', async () => {
  const { revocable, lease } = leased();
  let backendCalls = 0;
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({
      createSession: async () => { backendCalls += 1; return { sessionId: 's-1' }; },
    }),
    lease: () => lease,
    newRequestId: () => 'req-1',
  });
  revocable.revoke();
  await assert.rejects(office.start(goal), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  assert.equal(backendCalls, 0);
});

test('a scope revoked mid-start rejects the late receipt', async () => {
  const { revocable, lease } = leased();
  let releaseSession!: (value: { sessionId: string }) => void;
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({ createSession: () => new Promise<{ sessionId: string }>((resolve) => { releaseSession = resolve; }) }),
    lease: () => lease,
    newRequestId: () => 'req-1',
  });
  const pending = office.start(goal);
  // office.start 的首个 await 是 intentLog.load（顺序语义第 2 步），createSession 在其后；
  // 让出一个微任务使 createSession 已发起但未完成——revoke 仍发生在它落地之前（mid-start）。
  await Promise.resolve();
  revocable.revoke();
  releaseSession({ sessionId: 's-1' });
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('reconcilePending restores unresolved submissions from the durable intent log after a restart', async () => {
  const sharedLog = createInMemoryIntentLog();
  const { revocable, lease } = leased();
  await createTaskOffice({
    backend: createScenarioTaskBackend({ start: async () => { throw new Error('request lost'); } }),
    lease: () => lease,
    intentLog: sharedLog,
    newRequestId: () => 'req-restart',
  }).start(goal);
  // 模拟 App 重启：进程内 store 随内存丢失，耐久意图日志跨实例共享，新 office
  const second = createTaskOffice({
    backend: createScenarioTaskBackend({ lookup: async () => ({ state: 'admitted', run_id: 'run-recovered' }) }),
    lease: () => lease,
    intentLog: sharedLog,
    newRequestId: () => 'req-unused',
  });
  const receipts = await second.reconcilePending();
  assert.equal(receipts.length, 1);
  assert.equal(receipts[0]!.requestId, 'req-restart');
  assert.equal(receipts[0]!.phase, 'bound');
  assert.equal(receipts[0]!.runId, 'run-recovered');
  assert.equal(receipts[0]!.dispatched, false);
  assert.equal((await sharedLog.listScope(scope)).length, 0, 'bound 意图记录被清理，列表不无限增长');
  revocable.revoke();
});

test('after a restart, retrying with the same request id reuses the original session and resubmits only on unknown', async () => {
  const sharedLog = createInMemoryIntentLog();
  const { lease } = leased();
  await createTaskOffice({
    backend: createScenarioTaskBackend({ start: async () => { throw new Error('request lost'); } }),
    lease: () => lease,
    intentLog: sharedLog,
    newRequestId: () => 'req-1',
  }).start(goal);
  // 重启：新 office、新进程内 store；lookup 明确 unknown → 同 ID 同 session 重发
  const posts: TaskBackendStartInput[] = [];
  const secondBackend = createScenarioTaskBackend({
    lookup: async () => ({ state: 'unknown' }),
    start: async (input) => { posts.push(input); return { run_id: 'run-9', request_id: input.request_id, status: 'queued' }; },
  });
  const second = createTaskOffice({ backend: secondBackend, lease: () => lease, intentLog: sharedLog, newRequestId: () => 'req-unused' });
  const receipt = await second.start(goal, { requestId: 'req-1' });
  assert.equal(receipt.phase, 'bound');
  assert.equal(receipt.runId, 'run-9');
  assert.equal(secondBackend.calls.filter((call) => call.kind === 'createSession').length, 0, 'a retry NEVER creates a second session');
  assert.equal(posts.length, 1);
  assert.equal(posts[0]!.request_id, 'req-1');
  assert.equal(posts[0]!.session_id, 'session-scenario-1', 'the original session id is restored from the intent log');
  assert.equal((await sharedLog.listScope(scope)).length, 0, 'the intent record is cleaned up once bound');
});

test('replaying a bound intent after its log record was removed returns the bound receipt with zero network', async () => {
  const posts: string[] = [];
  const sessions: string[] = [];
  const { office } = startOffice({
    createSession: async () => { sessions.push('created'); return { sessionId: 'session-original' }; },
    start: async (input) => { posts.push(input.request_id); return { run_id: 'run-original', request_id: input.request_id, status: 'queued' }; },
    lookup: async () => ({ state: 'admitted', run_id: 'run-original' }),
  }, { ids: ['req-1'] });
  const first = await office.start(goal);
  assert.equal(first.phase, 'bound');
  // in-memory intentLog 的 remove 已在 bound 后执行——此刻记录缺失（B3-F78 前提）。
  const second = await office.start(goal, { requestId: 'req-1' });
  assert.equal(second.phase, 'bound');
  assert.equal(second.runId, 'run-original');
  assert.equal(second.dispatched, false);
  assert.equal(posts.length, 1, '幂等重放零新 POST');
  assert.equal(sessions.length, 1, '幂等重放零 createSession——绝不换 session');
});

test('a failing intentLog remove after a bound start no longer masquerades as failure', async () => {
  const log = createInMemoryIntentLog();
  const originalRemove = log.remove!;
  let failNextRemove = true;
  log.remove = async (requestId: string) => {
    if (failNextRemove) { failNextRemove = false; throw new Error('secure store busy'); }
    return originalRemove(requestId);
  };
  const { office } = startOffice({}, { log });
  const receipt = await office.start(goal); // B3-F64：remove 抛错不得把 bound 成功伪装成失败
  assert.equal(receipt.phase, 'bound');
  assert.equal(receipt.runId, 'run-scenario-1');
});

test('a bound start invalidates in-flight reads and the accumulated list cache', async () => {
  let releaseFirst!: (page: { items: [] }) => void;
  let calls = 0;
  const { office } = startOffice({
    list: async () => {
      calls += 1;
      if (calls === 1) return new Promise((resolve) => { releaseFirst = resolve; }); // 首次：在途慢读
      return { items: [] };
    },
  });
  const pending = office.tasks({}); // 在途读先发出（慢网络）
  const first = await office.start(goal); // bound 发生在在途读 settle 之前
  assert.equal(first.phase, 'bound');
  releaseFirst({ items: [] }); // 迟到的旧响应落地——epoch 已失配，必须被作废
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED',
    'B3-F79（对照 followUp）：bound 后在途 tasks() 必须被作废');
  // 累积缓存也被作废：moreTasks 无活跃查询（而非以旧快照继续翻页）
  await assert.rejects(office.moreTasks(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_NO_ACTIVE_QUERY',
    'B3-F79：bound 后累积列表缓存必须作废');
  const page = await office.tasks({}); // 下次 tasks() 重新查询（新查询正常返回）
  assert.ok(Array.isArray(page.items));
  assert.equal(calls, 2, '重新查询真的再次访问了后端');
});

test('reconcilePending skips a poisoned record and keeps the rest of the batch', async () => {
  const log = createInMemoryIntentLog();
  await log.save({ requestId: 'req-poison', sessionId: 's-x', goal, scope, persistedAt: '2026-09-24T00:00:00Z' });
  await log.save({ requestId: 'req-good', sessionId: 's-y', goal, scope, persistedAt: '2026-09-24T00:00:01Z' });
  const { office } = startOffice({
    lookup: async (requestId: string) => {
      if (requestId === 'req-poison') throw new Error('backend 500 for this record');
      return { state: 'unknown' };
    },
  }, { log });
  const receipts = await office.reconcilePending(); // B3-F65：毒记录不得让整批恢复 reject
  assert.ok(receipts.some((receipt) => receipt.requestId === 'req-good'));
  assert.ok(!receipts.some((receipt) => receipt.requestId === 'req-poison'), '毒记录被跳过，不出现在回执中');
});

// T10（#40）AC3 收口（final review critical）：该路径此前仅 opt-in live 冒烟覆盖——
// guardTaskBackend 只拦 Start POST，离线新意图先打未拦截的 createSession，以 transport
// 错误伪装成 TASK_OFFICE_BACKEND（cause 非 OfflineGateError），结构化 'run' 判决不可达。
// office 级 gate 端口使判决在最高稳定 Interface（office.start）直接可达。
test('an offline start (new intent) is refused at the office gate with a structured run verdict and zero backend dispatch', async () => {
  const { revocable, lease } = leased();
  let online = true;
  const gate = createOfflineGate({ online: async () => online });
  const backend = createScenarioTaskBackend({
    // transport 层故障按网络状态注入（smoke 同款 fetcher 语义）：离线时 createSession 若
    // 真触网必然失败——gate 缺席时这就是把 'run' 伪装成 TASK_OFFICE_BACKEND 的故障形状。
    createSession: async () => {
      if (!online) throw new Error('offline: network unreachable');
      return { sessionId: 'session-recovered' };
    },
  });
  const office = createTaskOffice({
    backend: guardTaskBackend(backend, gate), // 组合根同款：端口级 Start POST guard 仍在
    gate,
    lease: () => lease,
  });
  online = false; // 断网（smoke 同款注入语义）
  await assert.rejects(
    office.start(goal),
    (error: unknown) => error instanceof OfflineGateError && error.action === 'run',
    'the structured OFFLINE_ACTION_BLOCKED:run verdict must surface unwrapped at office.start',
  );
  assert.equal(backend.calls.length, 0, 'the refusal must precede even the pre-Start createSession');
  online = true; // 恢复联网：同一 office 正常提交
  const receipt = await office.start(goal);
  assert.equal(receipt.phase, 'bound');
  revocable.revoke();
});

test('an offline replay of an already-bound request still returns the zero-network receipt', async () => {
  const { revocable, lease } = leased();
  let online = true;
  const gate = createOfflineGate({ online: async () => online });
  const backend = createScenarioTaskBackend({});
  const office = createTaskOffice({
    backend: guardTaskBackend(backend, gate),
    gate,
    lease: () => lease,
    newRequestId: () => 'req-1',
  });
  const first = await office.start(goal); // 联网 bound
  assert.equal(first.phase, 'bound');
  online = false; // 断网：幂等重放零网络，不得被 gate 拒绝（它不派发任何新东西）
  const replay = await office.start(goal, { requestId: 'req-1' });
  assert.equal(replay.phase, 'bound');
  assert.equal(replay.dispatched, false);
  assert.equal(backend.calls.filter((call) => call.kind === 'start').length, 1, 'no second POST');
  assert.equal(backend.calls.filter((call) => call.kind === 'createSession').length, 1, 'no second session');
  revocable.revoke();
});
