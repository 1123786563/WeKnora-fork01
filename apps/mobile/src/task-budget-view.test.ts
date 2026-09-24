import test from 'node:test';
import assert from 'node:assert/strict';
import { createTaskBudgetController, TASK_BUDGET_COPY } from './task-budget-view.ts';
import { TaskBudgetError } from '@weknora/mobile-core';

type OfficeBudgetFace = Pick<import('@weknora/mobile-core').TaskOffice, 'budget' | 'extendBudget'>;

function officeWith(overrides: Partial<Record<'budget' | 'extendBudget', unknown>> = {}): OfficeBudgetFace {
  return {
    budget: overrides.budget as OfficeBudgetFace['budget'] ??
      (async () => ({
        taskId: 'r1', rootRunId: 'r1',
        limitCredits: 1000, usedCredits: 400, heldCredits: 100, remainingCredits: 500,
        delegatedRunIds: ['c1'], pausedRunIds: ['r1'], canExtend: true,
      })),
    extendBudget: overrides.extendBudget as OfficeBudgetFace['extendBudget'] ??
      (async () => ({ additionalCredits: 10, resumedRuns: 2 })),
  };
}

test('refresh loads the four numbers and the paused/delegated projection', async () => {
  const controller = createTaskBudgetController(officeWith(), { taskId: 'r1' });
  await controller.refresh();
  const state = controller.state();
  assert.equal(state.loading, false);
  assert.equal(state.error, undefined);
  assert.equal(state.facts?.limitCredits, 1000);
  assert.equal(state.facts?.remainingCredits, 500);
  assert.equal(state.facts?.pausedRunIds.length, 1);
  controller.dispose();
});

test('extend succeeds once, shows the resume count, and refetches fresh numbers', async () => {
  const calls: number[] = [];
  const office = officeWith({
    budget: async () => ({
      taskId: 'r1', rootRunId: 'r1',
      limitCredits: 1000 + calls.length * 10, usedCredits: 400, heldCredits: 100,
      remainingCredits: 500 + calls.length * 10,
      delegatedRunIds: [], pausedRunIds: calls.length === 0 ? ['r1'] : [], canExtend: true,
    }),
    extendBudget: async (input: { additionalCredits: number }) => {
      calls.push(input.additionalCredits);
      return { additionalCredits: input.additionalCredits, resumedRuns: 2 };
    },
  });
  const controller = createTaskBudgetController(office, { taskId: 'r1' });
  await controller.refresh();
  await controller.extend(10);
  const state = controller.state();
  assert.deepEqual(calls, [10]);
  assert.equal(state.message, '已追加 10 额度，恢复 2 个运行；追加预算不等于外部操作批准。');
  assert.equal(state.facts?.limitCredits, 1010);
  assert.equal(state.facts?.pausedRunIds.length, 0, 'refetch reflects the resumed state');
  controller.dispose();
});

test('a typed refusal surfaces honest copy per code and keeps the last facts', async () => {
  const office = officeWith({
    extendBudget: async () => { throw new TaskBudgetError('TASK_BUDGET_FORBIDDEN'); },
  });
  const controller = createTaskBudgetController(office, { taskId: 'r1' });
  await controller.refresh();
  await controller.extend(10);
  const state = controller.state();
  assert.equal(state.error, TASK_BUDGET_COPY.TASK_BUDGET_FORBIDDEN);
  assert.equal(state.facts?.limitCredits, 1000, 'refused extend never mutates local numbers');
  controller.dispose();
});

test('invalid local input is rejected without touching the office', async () => {
  let extendCalls = 0;
  const office = officeWith({
    extendBudget: async () => { extendCalls += 1; return { additionalCredits: 1, resumedRuns: 0 }; },
  });
  const controller = createTaskBudgetController(office, { taskId: 'r1' });
  await controller.refresh();
  await controller.extend(0);
  await controller.extend(1.5);
  assert.equal(extendCalls, 0);
  assert.equal(controller.state().error, TASK_BUDGET_COPY.TASK_BUDGET_INVALID_INPUT);
  controller.dispose();
});
