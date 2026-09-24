import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createTaskOffice, TaskOfficeError } from '../index.ts';
import type { InboxItem, InteractionActionValue, InteractionBackendPort, ResolvedDecisionRecord, TaskBackendPort } from '../index.ts';

const ITEM: InboxItem = {
  interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval', argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '2026-09-24T00:00:00Z',
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

// leaseActive 判定 instanceof RuntimeScopeLease（scope-lease.ts:26-28）：
// 假 lease 必须用真实类铸造（与 task-office.test.ts 的 leased() 同模式）。
function leaseFixture(): { revocable: RuntimeScopeLease; lease: ScopeLease } {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

function codedError(code: string): Error {
  const error = new Error(code);
  (error as unknown as { code?: string }).code = code;
  return error;
}

function scriptedInteractions(script: {
  inboxItems?: InboxItem[];
  inboxError?: Error;
  decideSteps?: Array<{ record?: ResolvedDecisionRecord; error?: Error }>;
}): InteractionBackendPort & { decideCalls: Array<{ item: InboxItem; decisionId: string; action: InteractionActionValue }>; inboxCalls: number } {
  let decideIndex = 0;
  let inboxCalls = 0;
  const decideCalls: Array<{ item: InboxItem; decisionId: string; action: InteractionActionValue }> = [];
  const port: InteractionBackendPort & { decideCalls: typeof decideCalls; inboxCalls: number } = {
    get inboxCalls() { return inboxCalls; },
    get decideCalls() { return decideCalls; },
    async inbox() {
      inboxCalls += 1;
      if (script.inboxError) throw script.inboxError;
      return script.inboxItems ?? [ITEM];
    },
    async decide(input) {
      decideCalls.push(input);
      const steps = script.decideSteps ?? [{}];
      const step = steps[Math.min(decideIndex, steps.length - 1)]!;
      decideIndex += 1;
      if (step.error) throw step.error;
      return step.record ?? {
        interactionId: input.item.interactionId,
        runId: input.item.runId,
        kind: input.item.kind,
        decisionId: input.decisionId,
        action: input.action,
        argsHash: input.item.argsHash,
        expectedRevision: input.item.expectedRevision + 1,
      };
    },
  };
  return port;
}

const emptyBackend: TaskBackendPort = {
  overview: () => Promise.resolve({ needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '2026-09-24T00:00:00Z' }),
  list: () => Promise.resolve({ items: [] }),
  archive: () => Promise.resolve(),
  restore: () => Promise.resolve(),
};

test('inbox fails closed without the interactions port and maps backend failures', async () => {
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease });
  await assert.rejects(office.inbox(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INTERACTIONS_UNAVAILABLE');

  const failing = scriptedInteractions({ inboxError: new Error('network down') });
  const wired = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions: failing });
  await assert.rejects(wired.inbox(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BACKEND');
});

test('inbox without a live scope lease rejects with SCOPE_CHANGED', async () => {
  const interactions = scriptedInteractions({});
  const office = createTaskOffice({ backend: emptyBackend, lease: () => undefined, interactions });
  await assert.rejects(office.inbox(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  assert.equal(interactions.inboxCalls, 0, 'scope 无效时不得发起后端读');
});

test('a newer inbox read supersedes an in-flight older one', async () => {
  const gate = deferred<InboxItem[]>();
  let calls = 0;
  const interactions: InteractionBackendPort = {
    inbox: () => { calls += 1; return calls === 1 ? gate.promise : Promise.resolve([ITEM]); },
    decide: () => Promise.reject(new Error('not used')),
  };
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions });
  const first = office.inbox();
  const second = office.inbox();
  gate.resolve([ITEM]);
  await assert.rejects(first, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED');
  assert.equal((await second).items.length, 1);
});

test('decide validates the frozen kind-action matrix locally before any backend call', async () => {
  const interactions = scriptedInteractions({});
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions });
  await assert.rejects(office.decide({ item: { ...ITEM, kind: 'budget' }, action: 'approve' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  assert.equal(interactions.decideCalls.length, 0, '矩阵违规不发网络请求');
});

test('decide freezes the decision id: repeats replay the same durable identity (AC1 mobile)', async () => {
  const interactions = scriptedInteractions({});
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions });
  const first = await office.decide({ item: ITEM, action: 'approve' });
  const second = await office.decide({ item: ITEM, action: 'approve' });
  assert.equal(first.status, 'recorded');
  assert.equal(second.status, 'recorded');
  assert.equal(interactions.decideCalls.length, 2);
  assert.equal(interactions.decideCalls[0].decisionId, interactions.decideCalls[1].decisionId, '双击/重试复用同一 decision_id');
  assert.notEqual(interactions.decideCalls[0].decisionId, '');
});

test('a delivery-unknown outcome keeps the decision id and a retry converts it to recorded (AC2)', async () => {
  const interactions = scriptedInteractions({ decideSteps: [{ error: codedError('INTERACTION_DELIVERY_UNKNOWN') }, {}] });
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions });
  const first = await office.decide({ item: ITEM, action: 'approve' });
  assert.equal(first.status, 'delivery-unknown');
  assert.equal(first.status === 'delivery-unknown' && first.decisionId !== '', true, 'delivery-unknown 携带已冻结的 decision_id');
  const retry = await office.decide({ item: ITEM, action: 'approve' });
  assert.equal(retry.status, 'recorded');
  assert.equal(interactions.decideCalls[0].decisionId, interactions.decideCalls[1].decisionId, '重试重放同一 durable 身份');
});

test('superseded and gone outcomes report honestly instead of throwing', async () => {
  const superseded = scriptedInteractions({ decideSteps: [{ error: codedError('INTERACTION_SUPERSEDED') }] });
  const officeA = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions: superseded });
  const receiptA = await officeA.decide({ item: ITEM, action: 'approve' });
  assert.deepEqual(receiptA, { status: 'superseded', interactionId: 'i-1' });

  const gone = scriptedInteractions({ decideSteps: [{ error: codedError('INTERACTION_GONE') }] });
  const officeB = createTaskOffice({ backend: emptyBackend, lease: () => leaseFixture().lease, interactions: gone });
  const receiptB = await officeB.decide({ item: ITEM, action: 'approve' });
  assert.deepEqual(receiptB, { status: 'gone', interactionId: 'i-1' });
});

test('a decided interaction invalidates the home projection and late inbox reads (Review Focus 5)', async () => {
  const interactions = scriptedInteractions({});
  const gate = deferred<{ needsMe: []; running: []; recentlyCompleted: []; unreadNotifications: 0; asOf: string }>();
  const backend: TaskBackendPort = {
    ...emptyBackend,
    overview: () => gate.promise,
  };
  const office = createTaskOffice({ backend, lease: () => leaseFixture().lease, interactions });
  const lateHome = office.home();
  await office.decide({ item: ITEM, action: 'approve' });
  gate.resolve({ needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '2026-09-24T00:00:00Z' });
  await assert.rejects(lateHome, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED', '决定终态 bump home epoch：迟到首页读不得回填已决定行');
});

test('a decided interaction whose scope was revoked mid-flight reports SCOPE_CHANGED, not recorded', async () => {
  const interactions = scriptedInteractions({});
  const { revocable, lease } = leaseFixture();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const office = createTaskOffice({ backend: emptyBackend, lease: () => leaseRef.lease, interactions });
  const pending = office.decide({ item: ITEM, action: 'approve' });
  revocable.revoke(); // 决定在途时切租户/登出：lease 撤销
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});
