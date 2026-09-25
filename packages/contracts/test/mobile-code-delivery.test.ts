import test from 'node:test';
import assert from 'node:assert/strict';
import { parseCodeDeliveryRecord } from '../src/mobile/code-delivery.ts';
import { ContractError } from '../src/index.ts';

// 真实服务端 wire（codedelivery.DeliveryView json tag 逐字一致）。
const deliveredWire = {
  id: 'dlv-1', task_id: 's-1', run_id: 'run-1', state: 'delivered',
  repo: 'octocat/hello', baseline_sha: 'b0000000000000000000000000000000000000000',
  branch: 'weknora/task/s-1', commit_sha: 'c1', pr_number: 7,
  pr_url: 'https://github.com/octocat/hello/pull/7', remote_login: 'octocat',
  action_id: 'act-1', action_state: 'succeeded', digest: 'd1', approver: 'u1',
  failure: '', files: 2, created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:01:00Z',
};

test('delivered wire parses into the traceability record', () => {
  const record = parseCodeDeliveryRecord(deliveredWire);
  assert.equal(record.state, 'delivered');
  assert.equal(record.taskId, 's-1');
  assert.equal(record.commitSha, 'c1');
  assert.equal(record.prNumber, 7);
  assert.equal(record.approver, 'u1');
  assert.equal(record.files, 2);
});

test('prepared wire parses with optional receipts omitted', () => {
  const record = parseCodeDeliveryRecord({
    ...deliveredWire, state: 'prepared', commit_sha: '', pr_number: 0,
    pr_url: '', remote_login: '', approver: '',
  });
  assert.equal(record.state, 'prepared');
  assert.equal(record.commitSha, undefined);
  assert.equal(record.prNumber, undefined);
  assert.equal(record.approver, undefined);
});

test('unknown state fails closed', () => {
  assert.throws(() => parseCodeDeliveryRecord({ ...deliveredWire, state: 'merged' }), ContractError);
  assert.throws(() => parseCodeDeliveryRecord('nope'), ContractError);
  assert.throws(() => parseCodeDeliveryRecord({ ...deliveredWire, id: '' }), ContractError);
});
