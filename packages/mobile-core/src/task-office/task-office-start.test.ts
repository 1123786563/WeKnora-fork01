import test from 'node:test';
import assert from 'node:assert/strict';
import { createInMemorySubmissionStore, type SubmissionStore } from '@weknora/domain/mobile';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
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
