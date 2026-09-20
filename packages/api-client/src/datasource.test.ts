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

// Vue confirmRemoveCredentials issues DELETE /api/v1/datasource/:id/credentials/credentials
// (the route takes a field segment and only the `credentials` field exists —
// internal/router/routes_infra.go; frontend/src/api/datasource/index.ts:174).
// The typed client must expose it so callers do not feature-detect; backend
// failures (e.g. 404 when nothing is stored) propagate to the caller for its
// remove-failed copy.
test('removeCredentials issues DELETE on the credentials subresource and propagates failures', async () => {
  const requests: Array<{ method: string; path: string }> = [];
  const failing = createDataSourcesApi(async (request) => {
    requests.push({ method: request.method, path: request.path });
    throw new Error('404: no credentials configured');
  });
  await assert.rejects(failing.removeCredentials('ds/a'), /404/);
  assert.deepEqual(requests, [{ method: 'DELETE', path: '/api/v1/datasource/ds%2Fa/credentials/credentials' }]);

  const ok = createDataSourcesApi(async (request) => {
    assert.equal(request.method, 'DELETE');
    assert.equal(request.path, '/api/v1/datasource/ds%2Fa/credentials/credentials');
    return undefined;
  });
  await ok.removeCredentials('ds/a');
});

test('rejects malformed data source responses', async () => {
  const api = createDataSourcesApi(async () => [{ ...source, id: '' }]);
  await assert.rejects(api.list('kb-1'), /Invalid data source field: id/);
});

// Task 4's cooperative cancel: POST /datasource/:id/logs/:log_id/cancel answers
// 202 {"status":"cancel_requested"} (internal/handler/datasource.go
// CancelSyncLog, Admin-gated at routes_infra.go). The request layer folds every
// 2xx — 202 included — into the parsed body (client.ts treats
// status < 200 || >= 300 as the only failure), so cancelSyncLog just POSTs and
// checks the envelope; transport failures propagate to the caller.
test('cancelSyncLog POSTs the log cancel route and validates the 202 envelope', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createDataSourcesApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    return { status: 'cancel_requested' };
  });
  await api.cancelSyncLog('ds/a', 'log/9');
  assert.deepEqual(requests, [{ method: 'POST', path: '/api/v1/datasource/ds%2Fa/logs/log%2F9/cancel', body: {} }]);
});

test('cancelSyncLog propagates transport failures and rejects broken envelopes', async () => {
  const failing = createDataSourcesApi(async () => { throw new Error('404: sync log not found'); });
  await assert.rejects(failing.cancelSyncLog('ds-1', 'log-9'), /404/);

  const malformed = createDataSourcesApi(async () => ({ status: 'nope' }));
  await assert.rejects(malformed.cancelSyncLog('ds-1', 'log-9'), /Invalid data source sync cancel/);
});

// SP2-a Task 10: remove grows the dual-choice delete opts — purge_documents
// only when explicitly opted in (the backend honors only the exact "true"
// string, internal/handler/datasource.go DeleteDataSource), so absent opts,
// an empty opts object and purgeDocuments:false all keep the pre-SP2-a
// keep-documents promise on the same bare DELETE path.
test('remove keeps the bare DELETE by default and appends purge_documents=true only when opted in', async () => {
  const requests: Array<{ method: string; path: string }> = [];
  const api = createDataSourcesApi(async (request) => { requests.push({ method: request.method, path: request.path }); return undefined; });
  await api.remove('ds/a');
  await api.remove('ds/a', {});
  await api.remove('ds/a', { purgeDocuments: false });
  await api.remove('ds/a', { purgeDocuments: true });
  assert.deepEqual(requests, [
    { method: 'DELETE', path: '/api/v1/datasource/ds%2Fa' },
    { method: 'DELETE', path: '/api/v1/datasource/ds%2Fa' },
    { method: 'DELETE', path: '/api/v1/datasource/ds%2Fa' },
    { method: 'DELETE', path: '/api/v1/datasource/ds%2Fa?purge_documents=true' },
  ]);
});

// SP2-a Task 9/10: GET /datasource/:id/documents-count answers {"count": N}
// and drives the delete panel's "N synced documents" copy; the client
// unwraps the envelope and rejects anything that is not a count integer.
test('documentsCount GETs the count route and unwraps the envelope', async () => {
  const requests: Array<{ method: string; path: string }> = [];
  const api = createDataSourcesApi(async (request) => { requests.push({ method: request.method, path: request.path }); return { count: 7 }; });
  assert.equal(await api.documentsCount('ds/a'), 7);
  assert.deepEqual(requests, [{ method: 'GET', path: '/api/v1/datasource/ds%2Fa/documents-count' }]);
});

test('documentsCount rejects malformed envelopes and propagates failures', async () => {
  const malformed = createDataSourcesApi(async () => ({ count: '7' }));
  await assert.rejects(malformed.documentsCount('ds-1'), /Invalid data source documents count/);
  const failing = createDataSourcesApi(async () => { throw new Error('500: count failed'); });
  await assert.rejects(failing.documentsCount('ds-1'), /500/);
});

// SP2-b Task 7: POST /datasource/:id/reindex (internal/handler/datasource.go
// ReindexItems) schedules a scoped reindex run. It answers 202
// {"sync_log_id": "..."}; a repeated request_id while the first run is still
// queued is rejected 409 — the request layer throws ApiError for every
// non-2xx, so the duplicate surfaces as a rejection the caller maps to its
// duplicate-request toast. 202 folds into the parsed body like every 2xx
// (client.ts), so only the envelope needs validating here.
test('reindexItems POSTs external_ids + request_id and unwraps the 202 sync_log_id', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createDataSourcesApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    return { sync_log_id: 'log-77' };
  });
  assert.equal(await api.reindexItems('ds/a', ['doc/1', 'doc/2'], 'req-uuid'), 'log-77');
  assert.deepEqual(requests, [{
    method: 'POST', path: '/api/v1/datasource/ds%2Fa/reindex',
    body: { external_ids: ['doc/1', 'doc/2'], request_id: 'req-uuid' },
  }]);
});

test('reindexItems propagates the 409 duplicate and rejects broken envelopes', async () => {
  const failing = createDataSourcesApi(async () => { throw new Error('409: a reindex request with this request_id is already enqueued'); });
  await assert.rejects(failing.reindexItems('ds-1', ['doc/1'], 'req-uuid'), /409/);

  const malformed = createDataSourcesApi(async () => ({ sync_log_id: 7 }));
  await assert.rejects(malformed.reindexItems('ds-1', ['doc/1'], 'req-uuid'), /Invalid data source reindex/);
  const missing = createDataSourcesApi(async () => ({ status: 'ok' }));
  await assert.rejects(missing.reindexItems('ds-1', ['doc/1'], 'req-uuid'), /Invalid data source reindex/);
});

// The sync-log wire carries the capped per-item failure samples under
// result.errors (SP2-b: SyncItemError with code/params/message/title and the
// external_id the targeted retry posts back). The client parse must keep the
// row intact instead of dropping the unknown result field.
test('logs keep the result.errors failure samples with external ids', async () => {
  const api = createDataSourcesApi(async () => ({
    data: [{
      id: 'log-1', status: 'success',
      result: { failed: 2, errors: [
        { title: 'Doc A', external_id: 'doc/a', code: 'feishu_rate_limited', message: 'Feishu API rate limited; will retry on the next sync' },
        { code: 'targeted_unsupported', message: 'needs a normal sync' },
      ] },
    }],
  }));
  const logs = await api.logs('ds-1');
  assert.equal(logs.length, 1);
  const errors = logs[0]?.result?.errors ?? [];
  assert.equal(errors.length, 2);
  assert.equal(errors[0]?.external_id, 'doc/a');
  assert.equal(errors[0]?.code, 'feishu_rate_limited');
  assert.equal(errors[1]?.code, 'targeted_unsupported');
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
