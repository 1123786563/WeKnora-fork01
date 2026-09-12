import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  CRAFT_EVENT_KINDS,
  parseCraftEventPayload,
  parseCraftInputView,
  parseCraftPreviewTicket,
  parseCraftRunEvent,
  parseCraftRunView,
  parseCraftSessionCreated,
  parseCraftSessionPage,
  parseCraftVersionView,
  parseCraftVersionsPage,
  parseCraftWorkspaceView,
} from './index.ts';

const runView = { run_id: 'run-1', session_id: 's1', status: 'running', wait_reason: '', revision: 3, epoch: 1, seq: 8, capabilities: { engine_type: 'trpc', durable_recovery: true } };
const versionView = {
  id: 'v1', workspace_id: 'w1', run_id: 'run-1', kind: 'web',
  files: [{ path: 'index.html', ref: 'r', sha256: 'abc', mime: 'text/html', bytes: 12 }],
  checks: [{ name: 'build', status: 'passed', detail: '' }],
};
const workspaceView = {
  session_id: 's1', workspace_id: 'w1', kind: 'web', title: '站点', engine_type: 'trpc',
  workspace: { id: 'w1', session_id: 's1', user_id: 'u1', sandbox_id: 'sbx', generation: '3', runtime_digest: 'd', revision: 2 },
  active_run_id: 'run-1', pending_id: '', last_seq: 8, active_run: runView, current_version: versionView,
};

test('session create requires both ids', () => {
  const created = parseCraftSessionCreated({ session_id: 's1', workspace_id: 'w1', engine_type: 'trpc' });
  assert.equal(created.session_id, 's1');
  assert.equal(created.engine_type, 'trpc');
  assert.throws(() => parseCraftSessionCreated({ workspace_id: 'w1', engine_type: 'trpc' }), /session_id/);
  assert.throws(() => parseCraftSessionCreated({ session_id: 's1', engine_type: 'trpc' }), /workspace_id/);
});

test('session page validates kind and cursor', () => {
  const page = parseCraftSessionPage({ data: [{ session_id: 's1', workspace_id: 'w1', kind: 'web', title: 't', engine_type: 'trpc', updated_at: '2026-09-13T00:00:00Z' }], next_cursor: null });
  assert.equal(page.data.length, 1);
  assert.equal(page.next_cursor, null);
  assert.equal(parseCraftSessionPage({ data: [], next_cursor: 'cur' }).next_cursor, 'cur');
  assert.throws(() => parseCraftSessionPage({ data: [{ session_id: 's1', workspace_id: 'w1', kind: 'mystery', title: 't', engine_type: 'trpc', updated_at: 'x' }], next_cursor: null }), /kind/);
  assert.throws(() => parseCraftSessionPage({ data: [{ workspace_id: 'w1', kind: 'web', title: 't', engine_type: 'trpc', updated_at: 'x' }], next_cursor: null }), /session_id/);
});

test('run view rejects unknown status and invalid seq', () => {
  const run = parseCraftRunView(runView);
  assert.equal(run.status, 'running');
  assert.equal(run.pending_id, null);
  const waiting = parseCraftRunView({ ...runView, status: 'waiting_user', wait_reason: 'p1', pending_id: 'p1' });
  assert.equal(waiting.pending_id, 'p1');
  assert.throws(() => parseCraftRunView({ ...runView, status: 'exploded' }), /status/);
  assert.throws(() => parseCraftRunView({ ...runView, seq: -1 }), /seq/);
  assert.throws(() => parseCraftRunView({ ...runView, seq: 1.5 }), /seq/);
  assert.throws(() => parseCraftRunView({ ...runView, run_id: '' }), /run_id/);
});

test('version view validates files, checks and ids', () => {
  const version = parseCraftVersionView(versionView);
  assert.equal(version.files[0]?.path, 'index.html');
  assert.equal(version.checks[0]?.status, 'passed');
  assert.throws(() => parseCraftVersionView({ ...versionView, id: '' }), /id/);
  assert.throws(() => parseCraftVersionView({ ...versionView, workspace_id: '' }), /workspace_id/);
  assert.throws(() => parseCraftVersionView({ ...versionView, checks: [{ name: 'b', status: 'maybe', detail: '' }] }), /status/);
  assert.throws(() => parseCraftVersionView({ ...versionView, files: [{ path: '', ref: 'r', sha256: 'x', mime: 'text/html', bytes: 1 }] }), /path/);
  assert.throws(() => parseCraftVersionView({ ...versionView, files: [{ path: 'a', ref: 'r', sha256: 'x', mime: 'text/html', bytes: -2 }] }), /bytes/);
  const page = parseCraftVersionsPage({ data: [versionView], next_cursor: null });
  assert.equal(page.data.length, 1);
});

test('workspace view requires last_seq and run projection', () => {
  const view = parseCraftWorkspaceView(workspaceView);
  assert.equal(view.last_seq, 8);
  assert.equal(view.active_run?.run_id, 'run-1');
  assert.equal(view.current_version?.id, 'v1');
  const idle = parseCraftWorkspaceView({ ...workspaceView, active_run_id: '', active_run: undefined, current_version: undefined });
  assert.equal(idle.active_run_id, null);
  assert.equal(idle.active_run, null);
  assert.equal(idle.current_version, null);
  assert.throws(() => parseCraftWorkspaceView({ ...workspaceView, last_seq: -3 }), /last_seq/);
  assert.throws(() => parseCraftWorkspaceView({ ...workspaceView, workspace_id: '' }), /workspace_id/);
  assert.throws(() => parseCraftWorkspaceView({ ...workspaceView, active_run: { ...runView, status: 'nope' } }), /status/);
});

test('input and preview tickets reject missing identities', () => {
  const input = parseCraftInputView({ ref: 'kb://1', name: 'a.pdf', sha256: 'aa', bytes: 3, citation_id: 'c1' });
  assert.equal(input.ref, 'kb://1');
  assert.throws(() => parseCraftInputView({ name: 'a.pdf', sha256: 'aa', bytes: 3, citation_id: 'c1' }), /ref/);
  const ticket = parseCraftPreviewTicket({ url: 'https://p.example/p/tok/', expires_at: '2026-09-13T01:00:00Z', version_id: 'v1' });
  assert.equal(ticket.version_id, 'v1');
  assert.throws(() => parseCraftPreviewTicket({ expires_at: 'x', version_id: 'v1' }), /url/);
  assert.throws(() => parseCraftPreviewTicket({ url: 'https://p.example/p/tok/', expires_at: 'x' }), /version_id/);
});

test('run events require a legal server-assigned seq', () => {
  const event = parseCraftRunEvent({ seq: 9, type: 'craft', payload: { kind: 'delegation.text' } });
  assert.equal(event.seq, 9);
  assert.equal(event.attempt_id, null);
  assert.throws(() => parseCraftRunEvent({ type: 'craft', payload: {} }), /seq/);
  assert.throws(() => parseCraftRunEvent({ seq: '9', type: 'craft', payload: {} }), /seq/);
  assert.throws(() => parseCraftRunEvent({ seq: 9, payload: {} }), /type/);
});

test('craft event payloads: known kinds strict, foreign payloads null', () => {
  const payload = parseCraftEventPayload({ workspace_id: 'w1', delegation_id: 'd1', tool_call_id: 't1', kind: 'artifact.published', data: { version_id: 'v2' } });
  assert.equal(payload?.kind, 'artifact.published');
  assert.equal((payload?.data as Record<string, unknown>)['version_id'], 'v2');
  assert.equal(CRAFT_EVENT_KINDS.includes('workspace.unavailable'), true);
  // Foreign (non-craft) run payloads are not craft events: null, not an error.
  assert.equal(parseCraftEventPayload({ type: 'chat', text: 'hi' }), null);
  assert.equal(parseCraftEventPayload('nope'), null);
  // Craft-namespaced but unknown kinds are a contract violation, not silence.
  assert.throws(() => parseCraftEventPayload({ workspace_id: 'w1', kind: 'artifact.vanished' }), /kind/);
  // Known kind without workspace identity is rejected, never guessed.
  assert.throws(() => parseCraftEventPayload({ kind: 'delegation.text' }), /workspace_id/);
  // data defaults to an empty object when the kind carries no payload.
  assert.deepEqual(parseCraftEventPayload({ workspace_id: 'w1', kind: 'workspace.unavailable', data: { workspace_id: 'w1' } })?.data, { workspace_id: 'w1' });
});
