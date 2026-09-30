import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileTaskBudgetRemote } from './task-budget.ts';
import type { ClientRequest } from '../client.ts';

const factsData = {
  task_id: 'r1', root_run_id: 'r1',
  limit_credits: 1000, used_credits: 400, held_credits: 100, remaining_credits: 500,
  deadline: '2026-09-24T12:00:00Z',
  delegated_run_ids: ['c1'], paused_run_ids: ['r1'], can_extend: true,
};

function remoteWith(handler: (input: ClientRequest) => Promise<unknown>) {
  return createMobileTaskBudgetRemote({ origin: 'https://weknora.example.test', request: handler });
}

function apiError(status: number, code: string): Error {
  return Object.assign(new Error(code), { name: 'ApiError', status, code });
}

test('facts() GETs the budget endpoint and maps the wire row to semantic fields', async () => {
  let seen: ClientRequest | undefined;
  const remote = remoteWith(async (input) => {
    seen = input;
    return { success: true, data: factsData };
  });
  const row = await remote.facts('r1');
  assert.equal(seen?.method, 'GET');
  assert.equal(seen?.path, '/api/v1/commercial/tasks/r1/budget');
  assert.equal(row.remainingCredits, 500);
  assert.deepEqual(row.delegatedRunIds, ['c1']);
  assert.equal(row.canExtend, true);
});

test('extend() POSTs the idempotent body and returns the receipt', async () => {
  let seen: ClientRequest | undefined;
  const remote = remoteWith(async (input) => {
    seen = input;
    return { success: true, data: { task_id: 'r1', additional_credits: 10, resumed_runs: 2 } };
  });
  const receipt = await remote.extend({ taskId: 'r1', additionalCredits: 10, idempotencyKey: 'k-1' });
  assert.equal(seen?.method, 'POST');
  assert.equal(seen?.path, '/api/v1/commercial/tasks/r1/budget/extend');
  assert.deepEqual(seen?.body, { additional_credits: 10, idempotency_key: 'k-1' });
  assert.equal(receipt.resumedRuns, 2);
});

test('wire refusals translate to cross-package contract codes on error.code', async () => {
  const cases: Array<[number, string, string]> = [
    [403, 'BUDGET_FORBIDDEN', 'TASK_BUDGET_FORBIDDEN'],
    [403, 'BUDGET_UNAUTHORIZED', 'TASK_BUDGET_FORBIDDEN'],
    [404, 'TASK_BUDGET_NOT_FOUND', 'TASK_BUDGET_NOT_FOUND'],
    [409, 'BUDGET_INSUFFICIENT', 'TASK_BUDGET_INSUFFICIENT'],
    [409, 'TASK_BUDGET_EXPIRED', 'TASK_BUDGET_EXPIRED'],
    [400, 'INVALID_REQUEST', 'TASK_BUDGET_INVALID_INPUT'],
    [400, 'INVALID_BUDGET_REQUEST', 'TASK_BUDGET_INVALID_INPUT'],
  ];
  for (const [status, wireCode, expected] of cases) {
    const remote = remoteWith(async () => { throw apiError(status, wireCode); });
    await assert.rejects(() => remote.facts('r1'), (error: unknown) =>
      (error as unknown as { code?: string }).code === expected, `${status} ${wireCode}`);
    await assert.rejects(() => remote.extend({ taskId: 'r1', additionalCredits: 10, idempotencyKey: 'k' }), (error: unknown) =>
      (error as unknown as { code?: string }).code === expected, `${status} ${wireCode}`);
  }
  // 未知形态（如 500 非 ApiError）原样透传：不吞成假成功。
  const remote = remoteWith(async () => { throw new Error('socket exploded'); });
  await assert.rejects(() => remote.facts('r1'), /socket exploded/);
});

test('malformed success payloads fail closed instead of fabricating numbers', async () => {
  for (const data of [{}, { ...factsData, remaining_credits: 499 }, null]) {
    const remote = remoteWith(async () => ({ success: true, data }));
    await assert.rejects(() => remote.facts('r1'));
  }
  const badEnvelope = remoteWith(async () => ({ success: false, error: { code: 'X', message: 'm' } }));
  await assert.rejects(() => badEnvelope.facts('r1'), /success/);
});

test('the remote rejects an invalid deployment origin at construction', () => {
  assert.throws(() => createMobileTaskBudgetRemote({ origin: 'http://insecure.example.test', request: async () => ({}) }));
  assert.throws(() => createMobileTaskBudgetRemote({ origin: 'https://example.test/path', request: async () => ({}) }));
});
