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
  assert.deepEqual(seen[0].path, '/workbench/executions/run%2F1/interactions');
});

test('decide posts the frozen decision body and validates the matrix locally', async () => {
  const seen: ClientRequest[] = [];
  const api = createInteractionsApi(async (input) => {
    seen.push(input);
    return { success: true, data: { id: 'i-1', decision_id: 'd-1', kind: 'tool_approval', action: 'approve', args_hash: 'sha256:aa', expected_revision: 5 } };
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
