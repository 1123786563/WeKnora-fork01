import assert from 'node:assert/strict';
import test from 'node:test';

import { createNativeRunState, reduceNativeEvent, shouldApplySequence } from './native.ts';

const scope = { tenant_id: '9007199254740993', session_id: 'session-1', run_id: 'run-1' };

function event(overrides: Record<string, unknown> = {}): any {
  return {
    protocol: 'weknora.agent.v1', schema_version: 1, event_id: 'event-1', ...scope,
    attempt_id: 'attempt-1', seq: '1', kind: 'attempt.started', payload: {}, ...overrides,
  };
}

test('classifies consecutive, replayed, and skipped decimal sequences', () => {
  assert.equal(shouldApplySequence(7n, 7n), 'duplicate');
  assert.equal(shouldApplySequence(7n, 9n), 'resync');
  assert.equal(shouldApplySequence(7n, 8n), 'apply');
});

test('marks a sequence gap for resync without applying the skipped event', () => {
  const initial = createNativeRunState(scope);
  const state = reduceNativeEvent(initial, event({ seq: '2', kind: 'run.status', attempt_id: undefined, payload: { status: 'running' } }));
  assert.equal(state.cursor, 0n);
  assert.equal(state.resync_required, true);
  assert.equal(state.status, 'queued');
});

test('ignores a replayed sequence without changing the reduced state', () => {
  const initial = createNativeRunState(scope);
  const applied = reduceNativeEvent(initial, event({ kind: 'run.status', attempt_id: undefined, payload: { status: 'running' } }));
  const replay = reduceNativeEvent(applied, event({ kind: 'run.status', attempt_id: undefined, payload: { status: 'failed' } }));
  assert.equal(replay, applied);
  assert.equal(replay.status, 'running');
});

test('keeps text isolated to the active replacement attempt while advancing the replay cursor', () => {
  let state = createNativeRunState(scope);
  state = reduceNativeEvent(state, event());
  state = reduceNativeEvent(state, event({ event_id: 'text-1', seq: '2', kind: 'text.delta', payload: { text: 'old', offset: 0 } }));
  state = reduceNativeEvent(state, event({ event_id: 'replace', seq: '3', attempt_id: 'attempt-2', kind: 'attempt.replaced', payload: { replaces_attempt_id: 'attempt-1' } }));
  state = reduceNativeEvent(state, event({ event_id: 'late-old', seq: '4', attempt_id: 'attempt-1', kind: 'text.delta', payload: { text: ' stale', offset: 3 } }));
  state = reduceNativeEvent(state, event({ event_id: 'new-text', seq: '5', attempt_id: 'attempt-2', kind: 'text.delta', payload: { text: 'new', offset: 0 } }));
  assert.equal(state.cursor, 5n);
  assert.equal(state.active_attempt_id, 'attempt-2');
  assert.equal(state.text, 'new');
  assert.equal(state.attempts['attempt-1']?.text, 'old');
});

test('makes the replaced attempt inactive even when the replacement envelope is incomplete', () => {
  let state = createNativeRunState(scope);
  state = reduceNativeEvent(state, event());
  state = reduceNativeEvent(state, event({ event_id: 'replace', seq: '2', attempt_id: undefined, kind: 'attempt.replaced', payload: { replaces_attempt_id: 'attempt-1' } }));
  state = reduceNativeEvent(state, event({ event_id: 'late', seq: '3', kind: 'text.delta', payload: { text: 'ignored', offset: 0 } }));
  assert.equal(state.active_attempt_id, undefined);
  assert.equal(state.text, '');
  assert.equal(state.cursor, 3n);
});

test('drops old-scope responses without mutating the current run state', () => {
  const initial = createNativeRunState(scope);
  const result = reduceNativeEvent(initial, event({ tenant_id: 'other-tenant', kind: 'run.status', attempt_id: undefined, payload: { status: 'running' } }));
  assert.equal(result, initial);
});

test('records decisions, tool results, artifacts, errors, and a terminal status without mutating inputs', () => {
  const initial = createNativeRunState(scope);
  const frozen = Object.freeze(initial);
  let state = reduceNativeEvent(frozen, event({ event_id: 'planned', seq: '1', kind: 'tool.planned', payload: { call_id: 'call-1', plan_version: 1, tool_name: 'search' } }));
  state = reduceNativeEvent(state, event({ event_id: 'decision', seq: '2', kind: 'decision.required', payload: { pending: { pending_id: 'pending-1', detail_path: '/detail', revision: '4' }, call_id: 'call-1', plan_version: 1, args_hash: 'sha256:args', expires_at: '2026-09-20T12:00:00Z', wait_kind: 'tool_approval' } }));
  state = reduceNativeEvent(state, event({ event_id: 'result', seq: '3', kind: 'tool.result', payload: { call_id: 'call-1', outcome: { attempt_id: 'attempt-1', call_id: 'call-1', result_hash: 'sha256:result', effect: 'confirmed', is_error: false, truncated: false, content: null } } }));
  state = reduceNativeEvent(state, event({ event_id: 'artifact', seq: '4', attempt_id: undefined, kind: 'artifact.available', payload: { artifact: { id: 'artifact-1', media_type: 'text/plain', sha256: 'sha256:a', size_bytes: '9007199254740993' } } }));
  state = reduceNativeEvent(state, event({ event_id: 'error', seq: '5', attempt_id: undefined, kind: 'error', payload: { failure: { code: 'provider_error', message: 'retry later', retryable: true, effect: 'unknown' } } }));
  state = reduceNativeEvent(state, event({ event_id: 'finished', seq: '6', attempt_id: undefined, kind: 'run.status', payload: { status: 'succeeded' } }));
  assert.deepEqual(state.pending['pending-1'], { pending_id: 'pending-1', detail_path: '/detail', revision: '4' });
  assert.equal(state.tools['call-1']?.outcome?.result_hash, 'sha256:result');
  assert.equal(state.artifacts['artifact-1']?.size_bytes, '9007199254740993');
  assert.equal(state.error?.code, 'provider_error');
  assert.equal(state.terminal?.status, 'succeeded');
  assert.equal(frozen.cursor, 0n);
});
