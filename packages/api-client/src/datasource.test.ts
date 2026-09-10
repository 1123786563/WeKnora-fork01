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

test('rejects malformed data source responses', async () => {
  const api = createDataSourcesApi(async () => [{ ...source, id: '' }]);
  await assert.rejects(api.list('kb-1'), /Invalid data source field: id/);
});
