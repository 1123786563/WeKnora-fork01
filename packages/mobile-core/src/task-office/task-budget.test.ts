import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import {
  createTaskBudgetOps, TaskBudgetError,
  type TaskBudgetBackendPort, type TaskBudgetFacts,
} from './task-budget.ts';
import { createScenarioTaskBudgetBackend } from './in-memory-task-budget.ts';
import { createTaskOffice, TaskOfficeError } from './task-office.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: '7' });
  return { revocable, lease: revocable.asScopeLease() };
}

const facts: TaskBudgetFacts = {
  taskId: 'r1', rootRunId: 'r1',
  limitCredits: 1000, usedCredits: 400, heldCredits: 100, remainingCredits: 500,
  delegatedRunIds: ['c1'], pausedRunIds: ['r1'], canExtend: true,
};

function opsWith(backend: TaskBudgetBackendPort, leaseRef: { lease?: ScopeLease }) {
  return createTaskBudgetOps({ backend, lease: () => leaseRef.lease, newIdempotencyKey: () => `idem-${Math.random().toString(36).slice(2)}` });
}

test('budget() reads the four numbers through the backend under a live lease', async () => {
  const backend = createScenarioTaskBudgetBackend({ facts });
  const { lease } = leased();
  const ops = opsWith(backend, { lease });
  const row = await ops.budget('r1');
  assert.equal(row.remainingCredits, 500);
  assert.deepEqual(row.pausedRunIds, ['r1']);
});

test('extendBudget keeps ONE idempotency key per logical extension and clears it after success', async () => {
  const seen: string[] = [];
  let failFirst = true;
  const backend: TaskBudgetBackendPort = {
    facts: async () => facts,
    extend: async (input) => {
      seen.push(input.idempotencyKey);
      if (failFirst) { failFirst = false; throw Object.assign(new Error('flaky transport'), { name: 'ApiError', status: 500 }); }
      return { additionalCredits: input.additionalCredits, resumedRuns: 2 };
    },
  };
  const { lease } = leased();
  const ops = opsWith(backend, { lease });
  await assert.rejects(() => ops.extendBudget({ taskId: 'r1', additionalCredits: 10 }), (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_BACKEND');
  const receipt = await ops.extendBudget({ taskId: 'r1', additionalCredits: 10 });
  assert.equal(receipt.resumedRuns, 2);
  assert.equal(seen.length, 2);
  assert.equal(seen[0], seen[1], 'retry of the same logical extension reuses the key (exactly-once server-side)');
  const receipt2 = await ops.extendBudget({ taskId: 'r1', additionalCredits: 10 });
  assert.equal(seen.length, 3);
  assert.notEqual(seen[2], seen[0], 'a NEW logical action gets a fresh key');
});

test('wire-coded refusals map to typed budget errors and never look like success', async () => {
  for (const [code, expected] of [
    ['TASK_BUDGET_FORBIDDEN', 'TASK_BUDGET_FORBIDDEN'],
    ['TASK_BUDGET_NOT_FOUND', 'TASK_BUDGET_NOT_FOUND'],
    ['TASK_BUDGET_INSUFFICIENT', 'TASK_BUDGET_INSUFFICIENT'],
    ['TASK_BUDGET_EXPIRED', 'TASK_BUDGET_EXPIRED'],
    ['TASK_BUDGET_INVALID_INPUT', 'TASK_BUDGET_INVALID_INPUT'],
  ] as const) {
    const backend: TaskBudgetBackendPort = {
      facts: async () => { throw codedError(code); },
      extend: async () => { throw codedError(code); },
    };
    const { lease } = leased();
    const ops = opsWith(backend, { lease });
    await assert.rejects(() => ops.budget('r1'), (error: unknown) => error instanceof TaskBudgetError && error.code === expected, code);
    await assert.rejects(() => ops.extendBudget({ taskId: 'r1', additionalCredits: 10 }), (error: unknown) => error instanceof TaskBudgetError && error.code === expected, code);
  }
});

function codedError(code: string): Error {
  const error = new Error(code);
  (error as unknown as { code?: string }).code = code;
  return error;
}

test('a revoked lease fails closed before dispatch and rejects late results (scope guard)', async () => {
  const { revocable, lease } = leased();
  let dispatched = 0;
  const backend: TaskBudgetBackendPort = {
    facts: async () => { dispatched += 1; return facts; },
    extend: async () => { dispatched += 1; return { additionalCredits: 10, resumedRuns: 0 }; },
  };
  const leaseRef = { lease: lease as ScopeLease | undefined };
  const ops = opsWith(backend, leaseRef);
  revocable.revoke();
  await assert.rejects(() => ops.budget('r1'), (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_SCOPE_CHANGED');
  await assert.rejects(() => ops.extendBudget({ taskId: 'r1', additionalCredits: 10 }), (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_SCOPE_CHANGED');
  assert.equal(dispatched, 0);
  // 迟到成功不越 scope 原样返回：dispatch 进行中撤销 lease → SCOPE_CHANGED。
  const again = leased();
  const slow: TaskBudgetBackendPort = {
    facts: async () => { await new Promise((resolve) => setTimeout(resolve, 5)); return facts; },
    extend: async () => ({ additionalCredits: 10, resumedRuns: 0 }),
  };
  const ops2 = createTaskBudgetOps({ backend: slow, lease: () => again.lease });
  const pending = ops2.budget('r1');
  again.revocable.revoke();
  await assert.rejects(() => pending, (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_SCOPE_CHANGED');
});

test('invalid input is rejected locally with zero backend calls', async () => {
  let calls = 0;
  const backend: TaskBudgetBackendPort = {
    facts: async () => { calls += 1; return facts; },
    extend: async () => { calls += 1; return { additionalCredits: 1, resumedRuns: 0 }; },
  };
  const { lease } = leased();
  const ops = opsWith(backend, { lease });
  for (const bad of [0, -5, 1.5, NaN]) {
    await assert.rejects(() => ops.extendBudget({ taskId: 'r1', additionalCredits: bad }),
      (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_INVALID_INPUT', `credits=${String(bad)}`);
  }
  await assert.rejects(() => ops.extendBudget({ taskId: '  ', additionalCredits: 10 }),
    (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_INVALID_INPUT');
  await assert.rejects(() => ops.budget(''), (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_INVALID_INPUT');
  assert.equal(calls, 0);
});

test('TaskOffice exposes budget()/extendBudget() and fails closed without the port', async () => {
  const office = createTaskOffice({ backend: createScenarioTaskBackend(), lease: () => leased().lease });
  await assert.rejects(() => office.budget('r1'), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BUDGET_UNAVAILABLE');
  await assert.rejects(() => office.extendBudget({ taskId: 'r1', additionalCredits: 10 }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BUDGET_UNAVAILABLE');

  const wired = createTaskOffice({
    backend: createScenarioTaskBackend(),
    lease: () => leased().lease,
    budget: createScenarioTaskBudgetBackend({ facts }),
  });
  const row = await wired.budget('r1');
  assert.equal(row.remainingCredits, 500);
  const receipt = await wired.extendBudget({ taskId: 'r1', additionalCredits: 10 });
  assert.equal(receipt.additionalCredits, 10);
});
