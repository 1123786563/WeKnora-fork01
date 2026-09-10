import assert from 'node:assert/strict';
import test from 'node:test';
import { ApiError } from './errors.ts';
import { createAdministrationApi } from './administration/index.ts';

test('maps system admin, API key scope, settings, and runtime cursor routes', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createAdministrationApi(async (request) => {
    requests.push(request);
    if (request.method === 'GET' && request.path === '/api/v1/system/admin/api-keys') return { success: true, data: [{ id: 1, name: 'platform', scope_type: 'platform', capabilities: ['system.settings.read'], api_key: '***', full_access: false, knowledge_base_ids: null, created_at: 'now' }] };
    if (request.path === '/api/v1/system/admin/runtime/queues') return { available: false, upstream_concurrency: 0, parse_concurrency: 0, wiki_concurrency: 0, pools: [], queues: [], model_limiter_available: false, models: [], timestamp: 1 };
    if (request.path === '/api/v1/system/admin/runtime/queues/default/tasks?state=archived&cursor=older&page_size=25') return { available: true, tasks: [], page_size: 25, has_more: false };
    if (request.path === '/api/v1/system/admin/audit-log?after_id=3&limit=20&outcome=success') return { success: true, data: [], next_cursor: 0 };
    if (request.path === '/api/v1/system/admin/settings') return [{ id: 1, key: 'x', value: true, value_type: 'bool', category: 'system', description: '', is_secret: false, requires_restart: false, last_modified_by: 'u', created_at: 'now', updated_at: 'now' }];
    if (request.path === '/api/v1/system/capabilities') return { code: 0, msg: 'success', data: { edition: 'lite', capabilities: { organizations: { supported: false, reason: 'not_supported_in_lite' } } } };
    return { success: true, data: { id: 1, name: 'new', scope_type: 'platform', capabilities: ['system.settings.read'], api_key: 'plain-once', full_access: false, knowledge_base_ids: null, created_at: 'now' } };
  });

  assert.equal((await api.apiKeys.list())[0]?.capabilities[0], 'system.settings.read');
  assert.equal((await api.runtime.queues()).available, false);
  assert.deepEqual(await api.runtime.tasks.list('default', 'archived', { cursor: 'older', pageSize: 25 }), { available: true, tasks: [], pageSize: 25, hasMore: false });
  assert.deepEqual(await api.auditLog.list({ afterId: 3, limit: 20, outcome: 'success' }), { items: [], nextCursor: 0 });
  assert.equal((await api.settings.list())[0]?.key, 'x');
  assert.deepEqual(await api.capabilities(), { edition: 'lite', capabilities: { organizations: { supported: false, reason: 'not_supported_in_lite' } } });
  assert.equal((await api.apiKeys.create({ name: 'new', capabilities: ['system.settings.read'] })).name, 'new');
  assert.deepEqual(requests.map((request) => [request.method, request.path]), [
    ['GET', '/api/v1/system/admin/api-keys'],
    ['GET', '/api/v1/system/admin/runtime/queues'],
    ['GET', '/api/v1/system/admin/runtime/queues/default/tasks?state=archived&cursor=older&page_size=25'],
    ['GET', '/api/v1/system/admin/audit-log?after_id=3&limit=20&outcome=success'],
    ['GET', '/api/v1/system/admin/settings'],
    ['GET', '/api/v1/system/capabilities'],
    ['POST', '/api/v1/system/admin/api-keys'],
  ]);
});

test('keeps system-admin authorization errors observable', async () => {
  const api = createAdministrationApi(async () => {
    throw new ApiError({ status: 403, code: 'forbidden', message: 'system admin required' });
  });
  await assert.rejects(() => api.settings.list(), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.status, 403);
    return true;
  });
});

test('maps API principal configuration and one-shot playground token routes', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createAdministrationApi(async (request) => {
    requests.push(request);
    if (request.method === 'GET') return { success: true, data: { mode: 'signed_token', direct_header_name: 'X-External-User-ID', signed_token_header_name: 'X-External-User-Token', require_direct_header: false, has_hmac_secret: true } };
    if (request.path.endsWith('api-principal-test-token')) return { token: 'jwt-once', header_name: 'X-External-User-Token', expires_in_seconds: 900, external_user_id: 'visitor-1' };
    return { success: true, data: { mode: 'direct_header', direct_header_name: 'X-External-User-ID', signed_token_header_name: 'X-External-User-Token', require_direct_header: true, has_hmac_secret: false } };
  });

  assert.equal((await api.tenantApiKeys.principalConfig(7)).mode, 'signed_token');
  assert.equal((await api.tenantApiKeys.updatePrincipalConfig(7, { mode: 'direct_header', requireDirectHeader: true })).require_direct_header, true);
  assert.deepEqual(await api.tenantApiKeys.createPrincipalTestToken(7, 'visitor-1'), { token: 'jwt-once', headerName: 'X-External-User-Token', expiresInSeconds: 900, externalUserId: 'visitor-1' });
  assert.deepEqual(requests.map(({ method, path, body }) => [method, path, body]), [
    ['GET', '/api/v1/tenants/7/api-principal-config', undefined],
    ['PUT', '/api/v1/tenants/7/api-principal-config', { mode: 'direct_header', require_direct_header: true }],
    ['POST', '/api/v1/tenants/7/api-principal-test-token', { external_user_id: 'visitor-1', expires_in_seconds: 900 }],
  ]);
});
