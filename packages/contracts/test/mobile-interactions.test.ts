import test from 'node:test';
import assert from 'node:assert/strict';
import { parseInteraction, parseInteractionDecision } from '../src/mobile/interactions.ts';
import { ContractError } from '../src/index.ts';

// 真实服务端 pending 交互 wire（Go interactionRow.decision() 序列化实测形状）：
// decision_id 与 action 均为空串——list 的核心场景（MX-005 审查 P1-1 回归）。
const pendingWire = {
  id: 'i1',
  decision_id: '',
  kind: 'tool_approval',
  action: '',
  args_hash: 'h',
  expected_revision: 0,
};

test('pending interaction wire parses with empty decision_id and action', () => {
  const record = parseInteraction(pendingWire);
  assert.equal(record.action, '');
  assert.equal(record.decision_id, '');
  assert.equal(record.kind, 'tool_approval');
});

test('decided interaction wire keeps the kind x action matrix', () => {
  const record = parseInteraction({ ...pendingWire, decision_id: 'd1', action: 'approve', expected_revision: 1 });
  assert.equal(record.action, 'approve');
  assert.throws(() => parseInteraction({ ...pendingWire, decision_id: 'd2', action: 'extend' }), (e: unknown) => e instanceof ContractError);
  // 已决定但 action 仍为空串：拒绝（不放宽）
  assert.throws(() => parseInteraction({ ...pendingWire, decision_id: 'd3', action: '' }), (e: unknown) => e instanceof ContractError);
});

test('decide body requires a concrete matrix action', () => {
  assert.equal(parseInteractionDecision({ ...pendingWire, decision_id: 'd1', action: 'approve' }).action, 'approve');
  assert.throws(() => parseInteractionDecision(pendingWire), /decision_id/);
  // 已决定但 action 仍为空串：在解析层即拒绝（信息为非空要求或具体动作要求）
  assert.throws(() => parseInteractionDecision({ ...pendingWire, decision_id: 'd1' }), /action/);
});
