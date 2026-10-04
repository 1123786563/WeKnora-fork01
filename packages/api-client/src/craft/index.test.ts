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

test('access API lists validated roles and posts grants and revocations through authenticated transport', async () => {
  const calls: ClientRequest[] = [];
  const signal = new AbortController().signal;
  const api = createCraftApi(async (input) => {
    calls.push(input);
    if (input.method === 'GET') return { success: true, data: [
      { user_id: 'owner/id', role: 'owner' },
      { user_id: 'viewer', role: 'viewer' },
    ] };
    return { success: true };
  });
  assert.deepEqual(await api.accessMembers('session / id', signal), [
    { user_id: 'owner/id', role: 'owner' },
    { user_id: 'viewer', role: 'viewer' },
  ]);
  await api.grantAccess('session / id', 'new-user', 'collaborator', signal);
  await api.revokeAccess('session / id', 'viewer', signal);
  assert.deepEqual(calls.map(({ method, path, body }) => ({ method, path, body })), [
    { method: 'GET', path: '/api/v1/sessions/session%20%2F%20id/craft/access', body: undefined },
    { method: 'POST', path: '/api/v1/sessions/session%20%2F%20id/craft/access', body: { user_id: 'new-user', role: 'collaborator' } },
    { method: 'POST', path: '/api/v1/sessions/session%20%2F%20id/craft/access/revoke', body: { user_id: 'viewer' } },
  ]);
  assert.ok(calls.every((call) => call.signal === signal), 'all methods pass the caller cancellation signal');
});

test('access API rejects malformed members and propagates forbidden and aborted requests', async () => {
  const malformed = createCraftApi(async () => ({ success: true, data: [{ user_id: 'viewer', role: 'admin' }] }));
  await assert.rejects(() => malformed.accessMembers('s1'), (error: unknown) => error instanceof ApiError && error.code === 'INVALID_RESPONSE');

  const forbidden = createCraftApi(async () => ({ success: false, error: { code: 'FORBIDDEN', message: 'Task access denied' } }));
  await assert.rejects(() => forbidden.accessMembers('s1'), (error: unknown) => error instanceof ApiError && error.code === 'FORBIDDEN');

  const controller = new AbortController();
  controller.abort();
  const aborted = createCraftApi(async ({ signal }) => {
    if (signal?.aborted) throw Object.assign(new Error('aborted'), { name: 'AbortError' });
    return { success: true, data: [] };
  });
  await assert.rejects(() => aborted.accessMembers('s1', controller.signal), (error: unknown) => error instanceof Error && error.name === 'AbortError');
});

test('input decisions post an exact ref and action and validate the acknowledged envelope', async () => {
  const calls: ClientRequest[] = [];
  const signal = new AbortController().signal;
  const api = createCraftApi(async (input) => {
    calls.push(input);
    return { success: true, data: { ref: 'opaque://accepted-1', action: 'continue' } };
  });
  assert.deepEqual(await api.decideInput('session / one', 'opaque://accepted-1', 'continue', signal), {
    ref: 'opaque://accepted-1', action: 'continue',
  });
  assert.deepEqual(calls, [{
    method: 'POST',
    path: '/api/v1/sessions/session%20%2F%20one/craft/inputs/decision',
    body: { ref: 'opaque://accepted-1', action: 'continue' },
    signal,
  }]);
});

test('input decision rejects a mismatched acknowledgement and preserves forbidden and abort errors', async () => {
  const mismatch = createCraftApi(async () => ({ success: true, data: { ref: 'opaque://other', action: 'continue' } }));
  await assert.rejects(() => mismatch.decideInput('s1', 'opaque://accepted-1', 'continue'), (error: unknown) => error instanceof ApiError && error.code === 'INVALID_RESPONSE');

  const forbidden = createCraftApi(async () => ({ success: false, error: { code: 'FORBIDDEN', message: 'decision denied' } }));
  await assert.rejects(() => forbidden.decideInput('s1', 'opaque://accepted-1', 'continue'), (error: unknown) => error instanceof ApiError && error.code === 'FORBIDDEN');

  const controller = new AbortController();
  controller.abort();
  const aborted = createCraftApi(async ({ signal }) => {
    if (signal?.aborted) throw Object.assign(new Error('aborted'), { name: 'AbortError' });
    return { success: true, data: { ref: 'opaque://accepted-1', action: 'continue' } };
  });
  await assert.rejects(() => aborted.decideInput('s1', 'opaque://accepted-1', 'continue', controller.signal), (error: unknown) => error instanceof Error && error.name === 'AbortError');
});

// T20 OCR: the edit panel's seam resolves the WHOLE PostCraftRun envelope —
// the envelope-level writer_acquisition and initiated_by must survive the
// api-client (unwrap would strip them down to data), both submit consumers
// ride ONE shared admission request builder, and a malformed envelope still
// fails closed.
test('submitEdit resolves the whole run envelope and shares the submit request builder', async () => {
  const envelope = {
    success: true,
    data: runView,
    writer_acquisition: { workspace_id: 'w1', status: 'acquired' },
    initiated_by: 'u-collaborator',
  };
  const { request, seen } = fakeRequest({
    'POST /api/v1/sessions/s1/craft/runs': envelope,
  });
  const api = createCraftApi(request);

  const raw = await api.submitEdit('s1', { request_id: 'req-edit-1', prompt: '把标题改成蓝色' });
  assert.deepEqual(raw, envelope, 'the envelope-level writer_acquisition and initiated_by survive');
  assert.equal(seen.length, 1);
  assert.equal(seen[0]?.method, 'POST');
  assert.equal(seen[0]?.path, '/api/v1/sessions/s1/craft/runs');
  assert.deepEqual(seen[0]?.body, {
    request_id: 'req-edit-1',
    prompt: '把标题改成蓝色',
    input_refs: [],
    knowledge_scope: '',
    base_version_id: '',
  });

  // The typed submit rides the SAME builder (identical path and body) with
  // its own RunView projection.
  const typed = await api.submit('s1', { request_id: 'req-edit-1', prompt: '把标题改成蓝色' });
  assert.equal(typed.run_id, 'run-1');
  assert.equal(seen.length, 2);
  assert.deepEqual(seen[1]?.body, seen[0]?.body, 'one contract, one request builder');

  // A malformed envelope fails closed instead of resolving garbage.
  const bad = fakeRequest({ 'POST /api/v1/sessions/s1/craft/runs': { success: false, error: { code: 'FORBIDDEN' } } });
  const badApi = createCraftApi(bad.request);
  await assert.rejects(() => badApi.submitEdit('s1', { request_id: 'req-edit-2', prompt: 'x' }), ApiError);
});

// Wrap-up OCR F01-F03: the consent endpoints answer a BARE flat body (no
// success/data envelope); unwrap-style reads would reject every successful
// consent response.
test('export consent seams resolve the bare flat wire bodies', async () => {
  const consentBody = {
    version_id: 'v1', manifest_digest: 'sha256:m', state: 'awaiting',
    restricted_derived: ['index.html'],
    files: [{ path: 'index.html', sha256: 'sha256:f', restricted: false, origins: [] }],
    decision: null,
  };
  const { request } = fakeRequest({
    'GET /api/v1/sessions/s1/craft/versions/v1/export/consent': consentBody,
    'POST /api/v1/sessions/s1/craft/versions/v1/export/consent/decision': { ...consentBody, state: 'consented' },
  });
  const api = createCraftApi(request);
  const view = await api.exportConsent('s1', 'v1');
  assert.deepEqual(view, consentBody, 'the bare flat consent body resolves as-is (no envelope rejection)');
  const decided = await api.decideExportConsent('s1', 'v1', 'approved', 'sha256:m');
  assert.equal((decided as Record<string, unknown>)['state'], 'consented', 'the decision body resolves after the server persisted it');
});

// T20 (#139) assembly: the T11 share seams follow the same bare-flat-body
// contract as the export-consent seams.
test('share consent seams resolve the bare flat wire bodies', async () => {
  const shareBody = { version_id: 'v1', restricted: true, evidence_digest: 'e'.repeat(64), status: 'private', decision: null };
  const decided = { ...shareBody, status: 'consented', decision: { version_id: 'v1', evidence_digest: 'e'.repeat(64), owner_id: 'owner', decision: 'approved' } };
  const { request, seen } = fakeRequest({
    'GET /api/v1/sessions/s1/craft/versions/v1/share': shareBody,
    'POST /api/v1/sessions/s1/craft/versions/v1/share/decision': decided,
    'POST /api/v1/sessions/s1/craft/versions/v1/share/revocation': shareBody,
  });
  const api = createCraftApi(request);
  const view = await api.shareView('s1', 'v1');
  assert.deepEqual(view, shareBody, 'the bare flat share body resolves as-is');
  const afterDecision = await api.decideShare('s1', 'v1', 'approved', 'e'.repeat(64));
  assert.equal((afterDecision as Record<string, unknown>)['status'], 'consented');
  assert.deepEqual(seen[1]?.body, { decision: 'approved', evidence_digest: 'e'.repeat(64) }, 'the decision posts the exact evidence digest');
  const afterRevoke = await api.revokeShare('s1', 'v1');
  assert.equal((afterRevoke as Record<string, unknown>)['status'], 'private');
});

test('budgetPause retains valid pause values when its optional extension action is malformed', async () => {
  const pause = { run_id: 'r1', reason: 'budget_exhausted', limit: 10, used: 10 };
  const { request } = fakeRequest({
    'GET /api/v1/sessions/s1/craft/runs/r1/budget/pause': {
      success: true,
      data: { ...pause, extension_action: { key: 'unsafe', extra_calls: Number.MAX_SAFE_INTEGER + 1, extra_credits: 10 } },
      can_extend: true,
    },
  });
  const api = createCraftApi(request);
  const view = await api.budgetPause('s1', 'r1');
  assert.deepEqual(view.pause, pause, 'the required pause remains available');
  assert.equal(view.extensionAction, null, 'invalid optional action is withheld');
});

test('budgetPause represents missing and null extension actions as null', async () => {
  const pause = { run_id: 'r1', reason: 'budget_exhausted', limit: 10, used: 10 };
  for (const body of [{ ...pause }, { ...pause, extension_action: null }]) {
    const { request } = fakeRequest({
      'GET /api/v1/sessions/s1/craft/runs/r1/budget/pause': { success: true, data: body },
    });
    const view = await createCraftApi(request).budgetPause('s1', 'r1');
    assert.deepEqual(view.pause, pause);
    assert.equal(view.extensionAction, null);
  }
});

test('budgetPause keeps required pause parsing strict when extension actions are optional', async () => {
  const { request } = fakeRequest({
    'GET /api/v1/sessions/s1/craft/runs/r1/budget/pause': {
      success: true,
      data: { run_id: 'r1', reason: 'budget_exhausted', limit: -1, used: 10, extension_action: null },
    },
  });
  const api = createCraftApi(request);
  await assert.rejects(() => api.budgetPause('s1', 'r1'), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.code, 'INVALID_RESPONSE');
    return true;
  });
});

test('budgetPause reads the server extension action from data and extend sends its exact tuple', async () => {
  const pause = { run_id: 'r1', reason: 'budget_exhausted', limit: 10, used: 10 };
  const action = { key: 'server-intent-key', extra_calls: 17, extra_credits: 2345678 };
  const { request, seen } = fakeRequest({
    'GET /api/v1/sessions/s1/craft/runs/r1/budget/pause': { success: true, data: { ...pause, extension_action: action }, can_extend: true },
    'POST /api/v1/sessions/s1/craft/runs/r1/budget/extend': { success: true },
  });
  const api = createCraftApi(request);
  const view = await api.budgetPause('s1', 'r1');
  assert.equal(view.pause.run_id, 'r1');
  assert.equal(view.pause.used, 10);
  assert.deepEqual(view.extensionAction, action, 'the server action is returned without substituting a client key or quantum');
  await api.extendBudget('s1', 'r1', view.extensionAction!);
  assert.deepEqual(seen[1]?.body, action, 'the POST echoes the exact server-owned tuple');
});
