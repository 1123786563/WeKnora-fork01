import assert from 'node:assert/strict';
import test from 'node:test';

import { approvalResolution, initialApprovalArgsDraft, parseApprovalArgsInput } from './tool-approval.tsx';

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
