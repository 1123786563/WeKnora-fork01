import test from 'node:test';
import assert from 'node:assert/strict';
import { createAppConnectorApi } from './appconnector.ts';
import { ApiError } from './errors.ts';
import type { ClientRequest } from './client.ts';

function fakeRequest(responses: Record<string, unknown>) {
  const seen: ClientRequest[] = [];
  const request = async (input: ClientRequest): Promise<unknown> => {
    seen.push(input);
    const key = input.method + ' ' + input.path;
    const body = responses[key];
    if (body === undefined) throw new Error('unexpected request: ' + key);
    return body;
  };
  return { request, seen };
}

test('client unwraps envelopes and carries expected_version in write bodies', async () => {
  const { request, seen } = fakeRequest({
    'GET /api/v1/apps/installations': { success: true, data: [{ id: 'i1', app_key: 'feishu', version: '1.0.0', scopes: ['doc:read'] }] },
    'POST /api/v1/apps/installations/i1/upgrade': { success: true, data: { id: 'i1', app_key: 'feishu', version: '2.0.0', scopes: ['doc:read', 'drive:read'] } },
    'POST /api/v1/apps/connections/c1/revoke': { success: true, data: { id: 'c1', kind: 'personal', state: 'revoked', owner_id: 'u1' } },
  });
  const api = createAppConnectorApi(request);
  const installations = await api.listInstallations();
  assert.equal(installations.length, 1);
  const upgraded = await api.upgradeInstallation('i1', { version: '2.0.0', expected_version: 4 });
  assert.equal(upgraded.version, '2.0.0');
  const revoked = await api.revokeConnection('c1', 9);
  assert.equal(revoked.state, 'revoked');
  assert.deepEqual(seen[1].body, { version: '2.0.0', expected_version: 4 });
  assert.deepEqual(seen[2].body, { expected_version: 9 });
});

test('connection responses NEVER expose credentials even when the payload leaks them', async () => {
  const { request } = fakeRequest({
    'GET /api/v1/apps/connections': {
      success: true,
      data: [{ id: 'c1', kind: 'space', state: 'active', owner_id: 'u1', access_token: 'LEAK', refresh_token: 'LEAK', credential_ref: 'LEAK' }],
    },
    'POST /api/v1/apps/connections': {
      success: true,
      data: {
        connection: { id: 'c2', kind: 'personal', state: 'active', owner_id: 'u1', credential: 'LEAK' },
        authorize_url: 'https://provider.example/authorize?state=abc',
      },
    },
  });
  const api = createAppConnectorApi(request);
  const listed = await api.listConnections();
  const listedJson = JSON.stringify(listed);
  assert.equal(listedJson.includes('access_token'), false);
  assert.equal(listedJson.includes('refresh_token'), false);
  assert.equal(listedJson.includes('credential'), false);
  assert.equal(listed.length, 1);

  const created = await api.createConnection({ installation_id: 'i1', kind: 'personal', expected_version: 1 });
  const createdJson = JSON.stringify(created);
  assert.equal(createdJson.includes('access_token'), false);
  assert.equal(createdJson.includes('refresh_token'), false);
  assert.equal(createdJson.includes('credential'), false);
  assert.equal(created.authorize_url.startsWith('https://provider.example/authorize'), true);
});

test('client surfaces envelope errors instead of returning data', async () => {
  const { request } = fakeRequest({
    'GET /api/v1/apps/datasources/ds1/sync-status': { success: false, error: { code: 'SYNC_BINDING_NOT_FOUND', message: 'no binding' } },
  });
  const api = createAppConnectorApi(request);
  await assert.rejects(() => api.getSyncStatus('ds1'), /no binding|SYNC_BINDING_NOT_FOUND/);
});

// ---- W05: action approval + task budget extension ----

test('approve body carries the action id (path), digest and expected_version', async () => {
  const { request, seen } = fakeRequest({
    'POST /api/v1/apps/actions/act_1/approve': {
      success: true,
      data: { action: { id: 'act_1', state: 'authorized', digest: 'd1', target: 'im chat', content: '{"a":1}', connection_name: 'space:c1' }, expected_version: 0 },
    },
  });
  const api = createAppConnectorApi(request);
  const detail = await api.approveAction('act_1', { digest: 'd1', expected_version: 0 });
  assert.equal(detail.action.state, 'authorized');
  assert.equal(seen[0].method, 'POST');
  assert.equal(seen[0].path, '/api/v1/apps/actions/act_1/approve');
  assert.deepEqual(seen[0].body, { digest: 'd1', expected_version: 0 });
});

test('prepare/execute/get hit the W05 action endpoints; budget extend is separate', async () => {
  const detail = { action: { id: 'act_2', state: 'awaiting_approval', digest: 'd2', target: 'notion page', content: '{"title":"t"}', connection_name: 'personal:c2' }, expected_version: 3 };
  const { request, seen } = fakeRequest({
    'POST /api/v1/apps/actions/prepare': { success: true, data: detail },
    'POST /api/v1/apps/actions/act_2/execute': { success: true, data: detail },
    'GET /api/v1/apps/actions/act_2': { success: true, data: detail },
    'POST /api/v1/commercial/tasks/task_9/budget/extend': { success: true, data: { task_id: 'task_9', additional_credits: 50 } },
  });
  const api = createAppConnectorApi(request);
  assert.equal((await api.prepareAction({ connection_id: 'c2', target: 'notion page', risk: 'send', content: '{"title":"t"}' })).action.id, 'act_2');
  assert.deepEqual(seen[0].body, { connection_id: 'c2', target: 'notion page', risk: 'send', content: '{"title":"t"}' });
  await api.executeAction('act_2');
  const got = await api.getAction('act_2');
  assert.equal(got.expected_version, 3);
  const ext = await api.extendTaskBudget('task_9', { additional_credits: 50, idempotency_key: 'idem-1' });
  assert.deepEqual(ext, { task_id: 'task_9', additional_credits: 50 });
  assert.equal(seen[3].path, '/api/v1/commercial/tasks/task_9/budget/extend');
  assert.deepEqual(seen[3].body, { additional_credits: 50, idempotency_key: 'idem-1' });
});

test('the api surface offers NO resend/retry-create path for unknown outcomes', async () => {
  const { request } = fakeRequest({
    'GET /api/v1/apps/actions/act_u': {
      success: true,
      data: { action: { id: 'act_u', state: 'unknown', digest: 'du', target: 'x', content: '{}', connection_name: 'space:c1' }, expected_version: 2 },
    },
  });
  const api = createAppConnectorApi(request);
  const detail = await api.getAction('act_u');
  assert.equal(detail.action.state, 'unknown');
  const surface = Object.keys(api).join(',').toLowerCase();
  for (const forbidden of ['resend', 'retry', 'recreate', 'requeue']) {
    assert.equal(surface.includes(forbidden), false, 'surface must not expose ' + forbidden + ': ' + surface);
  }
});

test('action envelope errors are typed ApiErrors', async () => {
  const { request } = fakeRequest({
    'POST /api/v1/apps/actions/act_1/approve': { success: false, error: { code: 'ACTION_DIGEST_MISMATCH', message: 'content changed' } },
  });
  const api = createAppConnectorApi(request);
  const rejection = api.approveAction('act_1', { digest: 'old', expected_version: 1 });
  await assert.rejects(rejection, (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal((error as ApiError).code, 'ACTION_DIGEST_MISMATCH');
    return true;
  });
});