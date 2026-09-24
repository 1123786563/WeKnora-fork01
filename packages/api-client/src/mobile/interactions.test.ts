import test from 'node:test';
import assert from 'node:assert/strict';
import { createInteractionsApi } from './interactions.ts';
import { ContractError } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

function respond(data: unknown) {
  return async (_input: ClientRequest): Promise<unknown> => ({ success: true, data });
}

test('interactions list unwraps and validates pending records', async () => {
  const seen: ClientRequest[] = [];
  const api = createInteractionsApi(async (input) => {
    seen.push(input);
    return {
      success: true,
      data: [
        // 真实服务端 pending wire：decision_id 与 action 均为空串
        { id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:aa', expected_revision: 4 },
      ],
    };
  });
  const items = await api.list('run/1');
  assert.equal(items.length, 1);
  assert.equal(items[0].kind, 'tool_approval');
  assert.equal(items[0].action, '');
  assert.equal(items[0].decision_id, '');
  assert.deepEqual(seen[0].path, '/api/v1/workbench/executions/run%2F1/interactions');
});

test('decide posts the frozen decision body and validates the matrix locally', async () => {
  const seen: ClientRequest[] = [];
  const api = createInteractionsApi(async (input) => {
    seen.push(input);
    return { success: true, data: { id: 'i-1', decision_id: 'd-1', kind: 'tool_approval', action: 'approve', args_hash: 'sha256:aa', expected_revision: 5, run_id: 'run-1' } };
  });
  const ack = await api.decide({ id: 'i-1', decision_id: 'd-1', kind: 'tool_approval', action: 'approve', args_hash: 'sha256:aa', expected_revision: 4 });
  assert.equal(ack.expected_revision, 5);
  assert.deepEqual(seen[0].body, { id: 'i-1', decision_id: 'd-1', kind: 'tool_approval', action: 'approve', args_hash: 'sha256:aa', expected_revision: 4 });

  // kind×action 互换在本地即拒绝（MX-003 冻结矩阵），不发起网络请求
  let sent = 0;
  const strict = createInteractionsApi(async () => {
    sent += 1;
    return { success: true, data: {} };
  });
  await assert.rejects(
    strict.decide({ id: 'i-2', decision_id: 'd-2', kind: 'budget', action: 'approve', args_hash: 'sha256:bb', expected_revision: 4 }),
    (error: unknown) => error instanceof ContractError,
  );
  assert.equal(sent, 0);
});

test('decide rejects empty decision_id before any request', async () => {
  const api = createInteractionsApi(respond({ id: 'x', decision_id: 'y', kind: 'recovery', action: 'retry', args_hash: 'z', expected_revision: 1 }));
  await assert.rejects(api.decide({ id: 'i-3', decision_id: '', kind: 'recovery', action: 'retry', args_hash: 'sha256:cc', expected_revision: 1 }), /decision_id/);
});

test('inbox unwraps pending rows across runs with run_id and validates limits', async () => {
  const seen: ClientRequest[] = [];
  const api = createInteractionsApi(async (input) => {
    seen.push(input);
    return {
      success: true,
      data: [
        { id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:aa', expected_revision: 4, run_id: 'run-1', created_at: '2026-09-24T00:00:00Z' },
        { id: 'i-2', decision_id: '', kind: 'recovery', action: '', args_hash: 'sha256:bb', expected_revision: 0, run_id: 'run-2' },
      ],
    };
  });
  const items = await api.inbox(25);
  assert.equal(items.length, 2);
  assert.equal(items[0].run_id, 'run-1');
  assert.equal(items[0].created_at, '2026-09-24T00:00:00Z');
  assert.equal(items[1].created_at, undefined);
  assert.deepEqual(seen[0].path, '/api/v1/workbench/interactions?limit=25');
  assert.deepEqual(seen[0].method, 'GET');

  // 缺 run_id 的行整包拒绝（收件箱行必须可导航）
  const broken = createInteractionsApi(async () => ({ success: true, data: [{ id: 'i-3', decision_id: '', kind: 'budget', action: '', args_hash: 'h', expected_revision: 0 }] }));
  await assert.rejects(broken.inbox(), (error: unknown) => error instanceof ContractError);

  // 非法 limit 本地拒绝，不发请求
  let sent = 0;
  const strict = createInteractionsApi(async () => { sent += 1; return { success: true, data: [] }; });
  await assert.rejects(strict.inbox(0), /limit/);
  await assert.rejects(strict.inbox(201), /limit/);
  assert.equal(sent, 0);
});
