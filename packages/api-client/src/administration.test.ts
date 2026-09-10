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
