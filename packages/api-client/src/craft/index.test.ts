import test from 'node:test';
import assert from 'node:assert/strict';
import { createCraftApi, craftDownloadPath } from './index.ts';
import { ApiError } from '../errors.ts';
import type { ClientRequest } from '../client.ts';

const runView = { run_id: 'run-1', session_id: 's1', status: 'queued', wait_reason: '', revision: 1, epoch: 1, seq: 0, capabilities: { engine_type: 'trpc', durable_recovery: true } };
const workspaceView = {
  session_id: 's1', workspace_id: 'w1', kind: 'web', title: 't', engine_type: 'trpc',
  workspace: { id: 'w1', session_id: 's1', user_id: 'u1', sandbox_id: 'sbx', generation: '1', runtime_digest: 'd', revision: 1 },
  active_run_id: 'run-1', pending_id: '', last_seq: 3, active_run: runView,
  current_version: { id: 'v1', workspace_id: 'w1', run_id: 'run-0', kind: 'web', files: [], checks: [] },
};

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

test('craft api hits the plan API table with exact bodies', async () => {
  const { request, seen } = fakeRequest({
    'POST /api/v1/craft/sessions': { success: true, data: { session_id: 's1', workspace_id: 'w1', engine_type: 'trpc' } },
    'GET /api/v1/craft/sessions?cursor=c&limit=20': { success: true, data: [], next_cursor: null },
    'GET /api/v1/sessions/s1/craft': { success: true, data: workspaceView },
    'POST /api/v1/sessions/s1/craft/inputs': { success: true, data: { ref: 'kb://1', name: 'a', sha256: 'x', bytes: 1, citation_id: 'c' } },
    'POST /api/v1/sessions/s1/craft/runs': { success: true, data: runView },
    'GET /api/v1/sessions/s1/craft/versions': { success: true, data: [], next_cursor: null },
    'GET /api/v1/sessions/s1/craft/versions/v1': { success: true, data: { id: 'v1', workspace_id: 'w1', run_id: 'run-0', kind: 'web', files: [], checks: [] } },
    'POST /api/v1/sessions/s1/craft/versions/v1/preview': { success: true, data: { url: 'https://p.example/p/tok/', expires_at: '2026-09-13T01:00:00Z', version_id: 'v1' } },
    'GET /api/v1/sessions/s1/craft/versions/v1/files/dist/index.html': '<html>bytes</html>',
  });
  const api = createCraftApi(request);
  const created = await api.create({ request_id: 'req-1', title: 'site', kind: 'web' });
  assert.equal(created.session_id, 's1');
  assert.deepEqual(await api.list({ cursor: 'c', limit: 20 }), { data: [], next_cursor: null });
  const view = await api.get('s1');
  assert.equal(view.last_seq, 3);
  assert.equal(view.active_run?.run_id, 'run-1');
  await api.addInput('s1', { resource_ref: 'kb://1', expected_sha256: 'x' });
  const run = await api.submit('s1', { request_id: 'req-2', prompt: 'hi' });
  assert.equal(run.status, 'queued');
  assert.deepEqual(seen[4]?.body, { request_id: 'req-2', prompt: 'hi', input_refs: [], knowledge_scope: '', base_version_id: '' });
  assert.equal((await api.versions('s1')).data.length, 0);
  assert.equal((await api.version('s1', 'v1')).id, 'v1');
  const ticket = await api.preview('s1', 'v1');
  assert.equal(ticket.version_id, 'v1');
  // Download returns the RAW transport body; bytes never become domain state.
  assert.equal(await api.download('s1', 'v1', 'dist/index.html'), '<html>bytes</html>');
});

test('download path matches the wildcard files/*file_path route and blocks traversal', () => {
  assert.equal(craftDownloadPath('s1', 'v1', 'dist/index.html'), '/api/v1/sessions/s1/craft/versions/v1/files/dist/index.html');
  assert.equal(craftDownloadPath('s 1', 'v1', 'a b/c.png'), '/api/v1/sessions/s%201/craft/versions/v1/files/a%20b/c.png');
  assert.throws(() => craftDownloadPath('s1', 'v1', ''), ApiError);
  assert.throws(() => craftDownloadPath('s1', 'v1', '../etc/passwd'), ApiError);
  assert.throws(() => craftDownloadPath('s1', 'v1', 'a' + String.fromCharCode(92) + 'b'), ApiError);
});

test('craft envelope errors surface as ApiError', async () => {
  const { request } = fakeRequest({
    'GET /api/v1/sessions/s1/craft': { success: false, error: { code: 'run_active', message: 'conflict', run_id: 'run-9' } },
  });
  const api = createCraftApi(request);
  await assert.rejects(() => api.get('s1'), (error: unknown) => error instanceof ApiError && error.code === 'run_active');
});

test('stop posts task_id and returns the honest phase', async () => {
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createCraftApi(async (input) => {
    calls.push({ method: input.method, path: input.path, body: input.body });
    return { success: true, data: { phase: 'stopping', note: 'abort accepted, still running' } };
  });
  const out = await api.stop('s1', 'run_1', 'dlg_1');
  if (out.phase !== 'stopping' || out.note !== 'abort accepted, still running') throw new Error('bad view: ' + JSON.stringify(out));
  if (calls[0]?.method !== 'POST' || calls[0]?.path !== '/api/v1/sessions/s1/craft/runs/run_1/stop') throw new Error('bad request');
  if (JSON.stringify(calls[0]?.body) !== JSON.stringify({ task_id: 'dlg_1' })) throw new Error('bad body');
});

test('delegationStatus polls the read-only endpoint', async () => {
  const api = createCraftApi(async (input) => {
    if (input.method !== 'GET' || input.path !== '/api/v1/sessions/s1/craft/runs/run_1/delegations/dlg_1/status') {
      throw new Error('unexpected request');
    }
    return { success: true, data: { phase: 'canceled', note: '' } };
  });
  const out = await api.delegationStatus('s1', 'run_1', 'dlg_1');
  if (out.phase !== 'canceled') throw new Error('bad phase');
});
