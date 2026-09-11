import test from 'node:test';
import assert from 'node:assert/strict';
import { createAppConnectorApi } from './appconnector.ts';
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
