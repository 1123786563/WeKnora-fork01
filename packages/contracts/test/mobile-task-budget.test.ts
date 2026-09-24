import test from 'node:test';
import assert from 'node:assert/strict';
import { parseTaskBudgetFacts, parseTaskBudgetExtension, ContractError } from '@weknora/contracts';

const factsWire = {
  task_id: 'r1', root_run_id: 'r1',
  limit_credits: 1000, used_credits: 400, held_credits: 100, remaining_credits: 500,
  deadline: '2026-09-24T12:00:00Z',
  delegated_run_ids: ['c1'], paused_run_ids: ['r1'], can_extend: true,
};

test('parseTaskBudgetFacts accepts the full wire row and keeps the four numbers distinct', () => {
  const row = parseTaskBudgetFacts(factsWire);
  assert.equal(row.task_id, 'r1');
  assert.equal(row.root_run_id, 'r1');
  assert.equal(row.limit_credits, 1000);
  assert.equal(row.used_credits, 400);
  assert.equal(row.held_credits, 100);
  assert.equal(row.remaining_credits, 500);
  assert.deepEqual(row.delegated_run_ids, ['c1']);
  assert.deepEqual(row.paused_run_ids, ['r1']);
  assert.equal(row.can_extend, true);
});

test('parseTaskBudgetFacts tolerates absent deadline and empty arrays', () => {
  const row = parseTaskBudgetFacts({ ...factsWire, deadline: undefined, delegated_run_ids: [], paused_run_ids: [] });
  assert.equal(row.deadline, undefined);
  assert.deepEqual(row.delegated_run_ids, []);
});

test('parseTaskBudgetFacts rejects arithmetic inconsistency and malformed numbers (Review Focus #2/#5 面)', () => {
  // 剩余与三数不一致：服务端序列化缺陷必须 fail-closed，客户端不得展示幻影数字。
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, remaining_credits: 499 }), ContractError);
  for (const bad of [1.5, NaN, '1000', -1, undefined]) {
    assert.throws(() => parseTaskBudgetFacts({ ...factsWire, limit_credits: bad }), ContractError,
      `limit_credits=${String(bad)}`);
  }
  for (const bad of [undefined, '', '   ', 7]) {
    assert.throws(() => parseTaskBudgetFacts({ ...factsWire, task_id: bad }), ContractError);
    assert.throws(() => parseTaskBudgetFacts({ ...factsWire, root_run_id: bad }), ContractError);
  }
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, delegated_run_ids: 'c1' }), ContractError);
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, delegated_run_ids: [''] }), ContractError);
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, paused_run_ids: [3] }), ContractError);
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, can_extend: 'yes' }), ContractError);
  assert.throws(() => parseTaskBudgetFacts(null), ContractError);
});

test('parseTaskBudgetExtension requires positive credits and a non-negative resume count', () => {
  const row = parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 10, resumed_runs: 2 });
  assert.equal(row.task_id, 'r1');
  assert.equal(row.additional_credits, 10);
  assert.equal(row.resumed_runs, 2);
  const zero = parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 10, resumed_runs: 0 });
  assert.equal(zero.resumed_runs, 0);
  assert.throws(() => parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 0, resumed_runs: 0 }), ContractError);
  assert.throws(() => parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 1.5, resumed_runs: 0 }), ContractError);
  assert.throws(() => parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 10, resumed_runs: -1 }), ContractError);
  assert.throws(() => parseTaskBudgetExtension({ task_id: '', additional_credits: 10, resumed_runs: 0 }), ContractError);
});
