import test from 'node:test';
import assert from 'node:assert/strict';
import { ContractError } from '../src/index.ts';
import { parseWorkbenchOverview, type WorkbenchOverview } from '../src/mobile/read-models.ts';

// 代表性 wire 字节（与 internal/modules/workbench/service/workbench/overview.go
// 的 JSON 标签逐字段对齐；真实序列化形状的固定样张）。
const overviewWire = {
  counts: { active_runs: 1, pending_interactions: 1, unread_notifications: 2 },
  in_progress: [{
    run_id: 'r1', session_id: 't1', title: 'weekly report', run_status: 'waiting_user',
    execution_status: 'waiting_user', settlement_status: 'pending', attention: 'required',
    updated_at: '2026-09-23T00:00:00Z',
  }],
  pending_interactions: [{ id: 'i1', kind: 'tool_approval', created_at: '2026-09-23T00:00:00Z' }],
  recently_completed: [{
    run_id: 'r2', session_id: 't2', title: 'finished research', run_status: 'succeeded',
    execution_status: 'succeeded', settlement_status: 'settled', attention: 'none',
    updated_at: '2026-09-22T00:00:00Z',
  }],
  recent_artifacts: [],
  as_of: '2026-09-23T00:00:01Z',
};

test('parseWorkbenchOverview maps the three home segments including recently completed', () => {
  const overview: WorkbenchOverview = parseWorkbenchOverview(overviewWire);
  assert.equal(overview.in_progress.length, 1);
  assert.equal(overview.in_progress[0]!.title, 'weekly report');
  assert.equal(overview.in_progress[0]!.attention, 'required');
  assert.equal(overview.recently_completed.length, 1);
  assert.equal(overview.recently_completed[0]!.run_id, 'r2');
  assert.equal(overview.recently_completed[0]!.settlement_status, 'settled');
  assert.equal(overview.counts.unread_notifications, 2);
});

test('attention is optional but validated when present; title falls back to empty string', () => {
  const withoutAttention = parseWorkbenchOverview({
    ...overviewWire,
    in_progress: [{ ...overviewWire.in_progress[0]!, title: undefined, attention: undefined }],
  });
  assert.equal(withoutAttention.in_progress[0]!.title, '');
  assert.equal(withoutAttention.in_progress[0]!.attention, undefined);

  assert.throws(() => parseWorkbenchOverview({
    ...overviewWire,
    in_progress: [{ ...overviewWire.in_progress[0]!, attention: 'urgent' }],
  }), (error: unknown) => error instanceof ContractError);
});

test('recently_completed is a required array, matching the server contract', () => {
  const { recently_completed: _omitted, ...withoutSegment } = overviewWire;
  assert.throws(() => parseWorkbenchOverview(withoutSegment), /recently_completed/);
  const empty = parseWorkbenchOverview({ ...overviewWire, recently_completed: [] });
  assert.deepEqual(empty.recently_completed, []);
});

test('overview rejects malformed envelopes without inventing data', () => {
  assert.throws(() => parseWorkbenchOverview({ counts: { active_runs: -1 } }), /active_runs/);
  assert.throws(() => parseWorkbenchOverview([]), /expected an object/);
});
