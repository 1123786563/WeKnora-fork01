import test from 'node:test';
import assert from 'node:assert/strict';
import { parseInteractionWithRun } from '@weknora/contracts';
import { ContractError } from '@weknora/contracts';

test('parseInteractionWithRun keeps the frozen interaction rules and requires run_id', () => {
  const row = parseInteractionWithRun({
    id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:aa',
    expected_revision: 4, run_id: 'run-1', created_at: '2026-09-24T00:00:00Z',
  });
  assert.equal(row.run_id, 'run-1');
  assert.equal(row.created_at, '2026-09-24T00:00:00Z');
  assert.equal(row.kind, 'tool_approval');

  // 冻结矩阵仍生效：budget 域携带 approve 拒绝
  assert.throws(() => parseInteractionWithRun({
    id: 'i-2', decision_id: 'd', kind: 'budget', action: 'approve', args_hash: 'h', expected_revision: 1, run_id: 'r',
  }), ContractError);

  // run_id 缺失/空串/非字符串：收件箱行不可导航、不可决定上下文，拒绝
  for (const runId of [undefined, '', '   ', 7]) {
    assert.throws(() => parseInteractionWithRun({
      id: 'i-3', decision_id: '', kind: 'recovery', action: '', args_hash: 'h', expected_revision: 0, run_id: runId,
    }), (error: unknown) => error instanceof ContractError, `run_id=${String(runId)}`);
  }

  // created_at 可选：老服务端不带该字段仍合法
  const bare = parseInteractionWithRun({
    id: 'i-4', decision_id: '', kind: 'budget', action: '', args_hash: 'h', expected_revision: 0, run_id: 'r2',
  });
  assert.equal(bare.created_at, undefined);
});
