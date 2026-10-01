import test from 'node:test';
import assert from 'node:assert/strict';
import { errorFromResult } from '../errors.ts';
import { createMobileCodeDeliveryRemote } from './code-delivery.ts';

const okDeliveryWire = {
  id: 'dlv-1', task_id: 's-1', run_id: 'run-1', state: 'pushed',
  repo: 'octocat/hello', baseline_sha: 'b0000000000000000000000000000000000000000',
  branch: 'weknora/task/s-1', commit_sha: 'c1', pr_number: 0, pr_url: '',
  remote_login: 'octocat', action_id: 'act-1', action_state: 'dispatched',
  digest: 'd1', approver: 'u1', failure: '', files: 1,
  created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:30Z',
};

function fakeRequest(reply: (input: any) => { status: number; body: unknown }) {
  const seen: any[] = [];
  return {
    seen,
    request: async (input: any) => {
      seen.push(input);
      const { status, body } = reply(input);
      if (status >= 400) {
        const err = new Error(`api error ${status}`) as any;
        err.status = status;
        err.body = body;
        throw err;
      }
      return { success: true, data: body };
    },
  };
}

test('delivery() maps GET /workbench/executions/:run/delivery onto the record', async () => {
  const transport = fakeRequest(() => ({ status: 200, body: { delivery: okDeliveryWire } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: transport.request });
  const record = await remote.delivery('run-1');
  assert.equal(record?.state, 'pushed');
  assert.equal(transport.seen[0].path, '/api/v1/workbench/executions/run-1/delivery');
  assert.equal(transport.seen[0].method, 'GET');
});

test('a 404 code_delivery_not_found maps to null, other failures reject', async () => {
  const notFound = fakeRequest(() => ({ status: 404, body: { code: 'code_delivery_not_found' } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: notFound.request });
  assert.equal(await remote.delivery('run-1'), null);

  const forbidden = fakeRequest(() => ({ status: 403, body: { code: 'code_delivery_forbidden' } }));
  const remote2 = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: forbidden.request });
  await assert.rejects(() => remote2.delivery('run-1'));
});

test('origin is validated at construction', () => {
  assert.throws(() => createMobileCodeDeliveryRemote({ origin: 'http://127.0.0.1:8080', request: async () => ({}) }));
});

test('a real ApiError-shaped 404 (top-level code, no body) also maps to null', async () => {
  // 生产通道形状：errorFromResult 产出的 ApiError 是顶层 .status/.code（errors.ts:60），
  // 无 .body —— 此路径若缺覆盖，实现退回只查 body.code 时本测试仍全绿（审查轮 1）。
  const apiErrorShape = async () => {
    const err = new Error('api error 404') as any;
    err.status = 404;
    err.code = 'code_delivery_not_found';
    throw err;
  };
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: apiErrorShape });
  assert.equal(await remote.delivery('run-1'), null);

  // 生产通道形状：errorFromResult 产出的 ApiError 是顶层 .status/.code（errors.ts:60），
  // 无 .body —— 404 携带其他 code 时不得映射为 null。
  const wrongCode = { seen: [] as any[], request: async () => {
    const err = new Error('api error 404') as any;
    err.status = 404;
    err.code = 'run_not_found';
    throw err;
  } };
  const remote2 = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: wrongCode.request });
  await assert.rejects(() => remote2.delivery('run-1'));
});

test('dispatchDelivery posts the PR-only recovery half and maps the record', async () => {
  const transport = fakeRequest(() => ({ status: 200, body: { delivery: okDeliveryWire } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: transport.request });
  const record = await remote.dispatchDelivery({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.equal(record.state, 'pushed');
  assert.equal(transport.seen[0].method, 'POST');
  assert.equal(transport.seen[0].path, '/api/v1/workbench/executions/run-1/delivery/dlv-1/dispatch');
});

test('resolveDelivery posts the unknown-resolution half and maps the record', async () => {
  const transport = fakeRequest(() => ({ status: 200, body: { delivery: okDeliveryWire } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: transport.request });
  const record = await remote.resolveDelivery({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.equal(record.state, 'pushed');
  assert.equal(transport.seen[0].method, 'POST');
  assert.equal(transport.seen[0].path, '/api/v1/workbench/executions/run-1/delivery/dlv-1/resolve');
});

test('a 409 state conflict rejects with the ApiError shape intact for recovery translation', async () => {
  const conflict = fakeRequest(() => ({ status: 409, body: { code: 'code_delivery_state_conflict' } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: conflict.request });
  await assert.rejects(
    () => remote.dispatchDelivery({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: any) => error.status === 409 && error.body?.code === 'code_delivery_state_conflict',
  );
});

test('production ApiError 409 conflicts propagate through both recovery methods', async () => {
  for (const method of ['dispatchDelivery', 'resolveDelivery'] as const) {
    const remote = createMobileCodeDeliveryRemote({
      origin: 'https://weknora.example.com',
      request: async () => { throw errorFromResult(409, { code: 'code_delivery_state_conflict' }); },
    });
    await assert.rejects(
      () => remote[method]({ runId: 'run-1', deliveryId: 'dlv-1' }),
      (error: any) => error.name === 'ApiError'
        && error.status === 409
        && error.code === 'code_delivery_state_conflict'
        && error.body === undefined,
    );
  }
});

test('recovery target 404 rejects instead of mapping to null', async () => {
  for (const method of ['dispatchDelivery', 'resolveDelivery'] as const) {
    const remote = createMobileCodeDeliveryRemote({
      origin: 'https://weknora.example.com',
      request: async () => { throw errorFromResult(404, { code: 'code_delivery_not_found' }); },
    });
    await assert.rejects(
      () => remote[method]({ runId: 'run-1', deliveryId: 'dlv-1' }),
      (error: any) => error.name === 'ApiError' && error.status === 404 && error.code === 'code_delivery_not_found',
    );
  }
});
