import assert from 'node:assert/strict';
import test from 'node:test';

import { approvalArgsStatus, approvalCountdownSeconds, approvalResolution, approvalTimerClass, formatApprovalCountdown, initialApprovalArgsDraft, parseApprovalArgsInput } from './tool-approval.tsx';

// Rendering assertions for ToolApprovalCard live in apps/web/src/chat/chat-page.test.ts,
// the only package in the workspace with react-dom available for SSR.

test('initialApprovalArgsDraft serializes the approval arguments', () => {
  assert.equal(initialApprovalArgsDraft({ arguments: { query: 'docs' } }), '{\n  "query": "docs"\n}');
  assert.equal(initialApprovalArgsDraft({}), '{}');
  assert.equal(initialApprovalArgsDraft({ arguments: undefined }), '{}');
});

test('parseApprovalArgsInput accepts a JSON object and an empty draft', () => {
  assert.deepEqual(parseApprovalArgsInput('{"query": "docs", "limit": 5}'), { ok: true, args: { query: 'docs', limit: 5 } });
  assert.deepEqual(parseApprovalArgsInput('   '), { ok: true, args: {} });
});

test('parseApprovalArgsInput rejects non-object JSON and malformed JSON', () => {
  const array = parseApprovalArgsInput('[1, 2]');
  assert.equal(array.ok, false);
  if (!array.ok) assert.match(array.error, /JSON object/);
  const scalar = parseApprovalArgsInput('42');
  assert.equal(scalar.ok, false);
  if (!scalar.ok) assert.match(scalar.error, /JSON object/);
  const broken = parseApprovalArgsInput('{"query": }');
  assert.equal(broken.ok, false);
  if (!broken.ok) assert.match(broken.error, /Invalid JSON/);
});

test('approvalResolution passes modified args through on approve', () => {
  const resolved = approvalResolution('approve', '{"query": "edited"}', true);
  assert.deepEqual(resolved, { ok: true, decision: 'approve', modifiedArgs: { query: 'edited' } });
});

test('approvalResolution rejects the flow without args and skips validation on reject', () => {
  assert.deepEqual(approvalResolution('reject', '{"query": }', true), { ok: true, decision: 'reject' });
  assert.deepEqual(approvalResolution('approve', '{"query": "docs"}', false), { ok: true, decision: 'approve' });
});

test('approvalResolution surfaces the parse error for invalid JSON on approve', () => {
  const resolved = approvalResolution('approve', 'not json', true);
  assert.equal(resolved.ok, false);
  if (!resolved.ok) assert.match(resolved.error, /Invalid JSON/);
});

test('approvalArgsStatus mirrors the Vue live validation contract', () => {
  // Vue ToolApprovalCard computes isJsonValid (empty draft counts as valid)
  // and argsDirty (trimmed draft differs from the initial args) on every edit.
  const initial = initialApprovalArgsDraft({ arguments: { query: 'docs' } });
  assert.deepEqual(approvalArgsStatus(initial, initial), { valid: true, dirty: false });
  assert.deepEqual(approvalArgsStatus('{"query": "edited"}', initial), { valid: true, dirty: true });
  assert.deepEqual(approvalArgsStatus('{"query": }', initial), { valid: false, dirty: true });
  assert.deepEqual(approvalArgsStatus('not json', '{}'), { valid: false, dirty: true });
  assert.deepEqual(approvalArgsStatus('', '{}'), { valid: true, dirty: true });
  assert.deepEqual(approvalArgsStatus('   ', initial), { valid: true, dirty: true });
});

test('approvalResolution carries the Vue user-rejected reason on reject', () => {
  const copy = { approvalInvalidJson: 'Invalid JSON', approvalArgsObject: 'Arguments must be a JSON object', approvalRejectedReason: '用户拒绝' };
  assert.deepEqual(approvalResolution('reject', '{"query": }', true, copy), { ok: true, decision: 'reject', reason: '用户拒绝' });
  assert.deepEqual(approvalResolution('reject', '', false), { ok: true, decision: 'reject' });
});

// Vue ToolApprovalCard countdown contract: deadline = (requestedAt || 0) * 1000
// + (timeoutSeconds || 600) * 1000, seconds floor-clamped at 0. Expiry never
// auto-rejects nor disables the buttons — the timer just sits at 0.
test('approvalCountdownSeconds mirrors the Vue deadline math and clamps at zero', () => {
  const now = 1_700_000_000_000;
  assert.equal(approvalCountdownSeconds(now, Math.floor(now / 1000) - 95, 120), 25);
  // Vue default timeoutSeconds is 600.
  assert.equal(approvalCountdownSeconds(now + 599_000, Math.floor(now / 1000)), 1);
  assert.equal(approvalCountdownSeconds(now + 600_000, Math.floor(now / 1000)), 0);
  // Expired requests stay pinned at 0 instead of going negative.
  assert.equal(approvalCountdownSeconds(now, Math.floor(now / 1000) - 3600, 600), 0);
  // Fractional seconds floor, mirroring Vue Math.floor((deadline - now) / 1000).
  assert.equal(approvalCountdownSeconds(now + 1_500, Math.floor(now / 1000), 30), 28);
});

// Vue timerClass: <= 30s is critical, <= 120s is warning, otherwise unstyled.
test('approvalTimerClass mirrors the Vue warning and critical thresholds', () => {
  assert.equal(approvalTimerClass(121), '');
  assert.equal(approvalTimerClass(120), 'wk-timer-warning');
  assert.equal(approvalTimerClass(31), 'wk-timer-warning');
  assert.equal(approvalTimerClass(30), 'wk-timer-critical');
  assert.equal(approvalTimerClass(0), 'wk-timer-critical');
});

// Vue formatCountdown: below 60s uses the localized agentStream.toolApproval
// countdownShort copy ({seconds} placeholder), otherwise m:ss with zero padding.
test('formatApprovalCountdown mirrors the Vue m:ss and short-form split', () => {
  const copy = { approvalCountdownShort: '{seconds}s' };
  assert.equal(formatApprovalCountdown(59, copy), '59s');
  assert.equal(formatApprovalCountdown(60, copy), '1:00');
  assert.equal(formatApprovalCountdown(605, copy), '10:05');
  assert.equal(formatApprovalCountdown(0, copy), '0s');
});
