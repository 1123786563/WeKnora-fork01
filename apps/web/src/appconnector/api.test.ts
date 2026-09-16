import test from 'node:test';
import assert from 'node:assert/strict';
import { createAppsApi } from './api.ts';

test('uses the backend catalog and authorization-attempt endpoints without fabricating URLs', async () => {
  const seen: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createAppsApi(async (input) => {
    seen.push(input);
    const responses: Record<string, unknown> = {
      'GET /api/v1/apps/catalog': { success: true, data: [{ action_id: 'github.list', app_id: 'github', app_version: '1', provider: 'github', risk: 'read', connection_id: 'c1', schema_digest: 'sha', input_schema: {}, required_scopes: ['repo:read'], published: true }] },
      'POST /api/v1/apps/connections/c1/authorization-attempts': { success: true, data: { attempt_id: 'attempt-1', status: 'pending', expires_at: '2026-09-16T00:00:00Z' } },
      'GET /api/v1/apps/authorization-attempts/attempt-1': { success: true, data: { attempt_id: 'attempt-1', status: 'active', connection_id: 'c1', expires_at: '2026-09-16T00:00:00Z' } },
      'GET /api/v1/apps/connections': { success: true, data: [{ id: 'c1', kind: 'space', state: 'active', owner_id: null, auth_version: 7 }] },
      'POST /api/v1/apps/connections/c1/revoke': { success: true, data: { id: 'c1', kind: 'space', state: 'revoked', owner_id: null, auth_version: 8 } },
    };
    return responses[input.method + ' ' + input.path];
  });
  const catalog = await api.listCatalog();
  assert.equal(catalog[0].action_id, 'github.list');
  const attempt = await api.beginAuthorization('c1');
  assert.deepEqual(attempt, { attempt_id: 'attempt-1', status: 'pending', connection_id: 'c1', expires_at: '2026-09-16T00:00:00Z' });
  assert.equal((await api.getAuthorization('attempt-1')).status, 'active');
  assert.equal((await api.listConnections())[0].auth_version, 7);
  assert.equal((await api.revokeConnection('c1', 7)).state, 'revoked');
  assert.deepEqual(seen.map(({ method, path, body }) => ({ method, path, body })), [
    { method: 'GET', path: '/api/v1/apps/catalog', body: undefined },
    { method: 'POST', path: '/api/v1/apps/connections/c1/authorization-attempts', body: {} },
    { method: 'GET', path: '/api/v1/apps/authorization-attempts/attempt-1', body: undefined },
    { method: 'GET', path: '/api/v1/apps/connections', body: undefined },
    { method: 'POST', path: '/api/v1/apps/connections/c1/revoke', body: { expected_version: 7 } },
  ]);
});

test('maps backend error envelopes and preserves abort signals', async () => {
  const signal = new AbortController().signal;
  const api = createAppsApi(async (input) => {
    assert.equal(input.signal, signal);
    return { success: false, error: { code: 'CONNECTION_FORBIDDEN', message: 'forbidden' } };
  });
  await assert.rejects(() => api.listConnections(signal), /forbidden/);
});
