import assert from 'node:assert/strict';
import test from 'node:test';

import { createDataSourcesApi } from './datasource.ts';

const source = { id: 'ds-1', knowledge_base_id: 'kb-1', name: 'Docs', type: 'notion' };

test('lists and creates data sources with tenant-scoped paths', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createDataSourcesApi(async (request) => {
    requests.push(request);
    return request.method === 'GET' ? [source] : source;
  });
  assert.equal((await api.list('kb/a'))[0]?.id, 'ds-1');
  assert.equal((await api.create({ knowledge_base_id: 'kb-1', name: 'Docs', type: 'notion' })).name, 'Docs');
  assert.equal(requests[0]?.path, '/api/v1/datasource?kb_id=kb%2Fa');
  assert.equal(requests[1]?.path, '/api/v1/datasource');
});

test('encodes resource ids and keeps sync controls as explicit writes', async () => {
  const requests: Array<{ method: string; path: string }> = [];
  const api = createDataSourcesApi(async (request) => { requests.push({ method: request.method, path: request.path }); return []; });
  await api.resources('ds/a', 'folder one');
  await api.sync('ds/a');
  await api.pause('ds/a');
  await api.resume('ds/a');
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/datasource/ds%2Fa/resources?parent_id=folder%20one' },
    { method: 'POST', path: '/api/v1/datasource/ds%2Fa/sync' },
    { method: 'POST', path: '/api/v1/datasource/ds%2Fa/pause' },
    { method: 'POST', path: '/api/v1/datasource/ds%2Fa/resume' },
  ]);
});

test('loads resource ancestors through the explicit restoration route', async () => {
  const api = createDataSourcesApi(async (request) => {
    assert.equal(request.method, 'POST');
    assert.equal(request.path, '/api/v1/datasource/ds%2Fa/resource-ancestors');
    assert.deepEqual(request.body, { resource_ids: ['page/1'] });
    return { ancestors: ['root', 'folder/1'] };
  });
  assert.deepEqual(await api.resourceAncestors('ds/a', ['page/1']), ['root', 'folder/1']);
});

test('rejects malformed data source responses', async () => {
  const api = createDataSourcesApi(async () => [{ ...source, id: '' }]);
  await assert.rejects(api.list('kb-1'), /Invalid data source field: id/);
});

test('exposes connector types, credential writes, and sync logs as separate routes', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createDataSourcesApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    if (request.path.endsWith('/types')) return [{ type: 'notion', name: 'Notion', description: 'Notion', priority: 1, auth_type: 'token', capabilities: ['incremental'] }, { type: 'rss', name: 'RSS', description: 'RSS', priority: 2, auth_type: 'none', capabilities: [] }];
    if (request.path.endsWith('/logs?limit=20&offset=0')) return { data: [{ id: 'log-1', status: 'success' }] };
    return { credentials: { configured: true } };
  });
  assert.deepEqual((await api.types()).map((type) => type.type), ['notion', 'rss']);
  await api.putCredentials('ds/a', { token: 'secret' });
  assert.deepEqual(await api.logs('ds/a'), [{ id: 'log-1', status: 'success' }]);
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/datasource/types', body: undefined },
    { method: 'PUT', path: '/api/v1/datasource/ds%2Fa/credentials', body: { credentials: { token: 'secret' } } },
    { method: 'GET', path: '/api/v1/datasource/ds%2Fa/logs?limit=20&offset=0', body: undefined },
  ]);
});
