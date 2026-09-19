import assert from 'node:assert/strict';
import test from 'node:test';
import {
  parseQueryHistoryConfigResponse,
  parseQueryHistoryExportStartResponse,
  parseQueryHistoryExportStatusResponse,
  parseQueryHistorySessionListResponse,
  parseQueryHistoryShareTokenResponse,
  parseQueryHistorySnapshotResponse,
  parseSharedSessionResponse,
} from './query-history.ts';

const sessionRow = {
  id: 's1',
  title: '审计会话',
  user_id: 'u1',
  engine_type: 'builtin',
  is_pinned: false,
  created_at: '2026-09-19T10:00:00Z',
  updated_at: '2026-09-19T10:05:00Z',
};
const messageRow = {
  id: 'm1',
  session_id: 's1',
  role: 'user',
  content: '今天天气如何',
  knowledge_references: [{ knowledge_id: 'k1' }],
};
const feedbackRow = {
  id: 7,
  tenant_id: 1,
  user_id: 'u1',
  message_id: 'm1',
  session_id: 's1',
  rating: 'like',
  comment: '有帮助',
  created_at: '2026-09-19T10:06:00Z',
  updated_at: '2026-09-19T10:06:00Z',
};
const snapshotEnvelope = {
  success: true,
  data: {
    session: sessionRow,
    messages: [messageRow],
    feedback: [feedbackRow],
    truncated: true,
  },
};

test('parseQueryHistorySessionListResponse unwraps the audit listing envelope', () => {
  const result = parseQueryHistorySessionListResponse({
    success: true,
    data: [sessionRow],
    total: 1,
    page: 0,
    page_size: 50,
  });
  assert.equal(result.data.length, 1);
  assert.equal(result.data[0]!.user_id, 'u1');
  assert.equal(result.data[0]!.engine_type, 'builtin');
  assert.equal(result.total, 1);
  assert.equal(result.page, 0);
  assert.equal(result.page_size, 50);
});

test('parseQueryHistorySessionListResponse narrows audit rows and pagination', () => {
  assert.throws(() => parseQueryHistorySessionListResponse({
    success: true,
    data: [{ ...sessionRow, user_id: 42 }],
    total: 1,
    page: 0,
    page_size: 50,
  }), /data\[0\]\.user_id/);
  assert.throws(() => parseQueryHistorySessionListResponse({
    success: true,
    data: [sessionRow],
    total: -1,
    page: 0,
    page_size: 50,
  }), /total/);
  assert.throws(() => parseQueryHistorySessionListResponse({
    success: true,
    data: 'nope',
    total: 1,
    page: 0,
    page_size: 50,
  }), /expected an array/);
});

test('parseQueryHistorySnapshotResponse validates session, messages, feedback, truncated', () => {
  const result = parseQueryHistorySnapshotResponse(snapshotEnvelope);
  assert.equal(result.session.id, 's1');
  assert.equal(result.messages[0]!.id, 'm1');
  assert.deepEqual(result.messages[0]!.knowledge_references, [{ knowledge_id: 'k1' }]);
  assert.equal(result.feedback[0]!.rating, 'like');
  assert.equal(result.truncated, true);
});

test('parseQueryHistorySnapshotResponse narrows feedback ratings and truncation', () => {
  assert.throws(() => parseQueryHistorySnapshotResponse({
    ...snapshotEnvelope,
    data: { ...snapshotEnvelope.data, feedback: [{ ...feedbackRow, rating: 'meh' }] },
  }), /rating/);
  assert.throws(() => parseQueryHistorySnapshotResponse({
    ...snapshotEnvelope,
    data: { ...snapshotEnvelope.data, truncated: 'yes' },
  }), /truncated/);
  assert.throws(() => parseQueryHistorySnapshotResponse({
    ...snapshotEnvelope,
    data: { ...snapshotEnvelope.data, session: null },
  }), /session/);
  assert.throws(() => parseQueryHistorySnapshotResponse({
    ...snapshotEnvelope,
    data: { ...snapshotEnvelope.data, messages: [{ ...messageRow, role: 'tool' }] },
  }), /role/);
});

test('parseSharedSessionSnapshot omits feedback and keeps the shared shape', () => {
  const result = parseSharedSessionResponse({
    success: true,
    data: { session: sessionRow, messages: [messageRow], truncated: false },
  });
  assert.equal(result.session.id, 's1');
  assert.equal(result.messages[0]!.content, '今天天气如何');
  assert.equal(result.truncated, false);
  assert.throws(() => parseSharedSessionResponse({
    success: true,
    data: { session: sessionRow, messages: {}, truncated: false },
  }), /messages/);
});

test('parseQueryHistoryExportStartResponse requires a job_id', () => {
  assert.deepEqual(parseQueryHistoryExportStartResponse({ success: true, data: { job_id: 3 } }), { job_id: 3 });
  assert.throws(() => parseQueryHistoryExportStartResponse({ success: true, data: {} }), /job_id/);
  assert.throws(() => parseQueryHistoryExportStartResponse({ success: true, data: { job_id: '3' } }), /job_id/);
});

test('parseQueryHistoryExportStatusResponse narrows status and requires error_message', () => {
  const result = parseQueryHistoryExportStatusResponse({
    success: true,
    data: { job_id: 3, status: 'failed', error_message: 'boom', file_path: '/tmp/x.csv' },
  });
  assert.equal(result.status, 'failed');
  assert.equal(result.error_message, 'boom');
  assert.equal(result.job_id, 3);
  assert.equal((result as { file_path?: unknown }).file_path, '/tmp/x.csv');
  assert.deepEqual(
    parseQueryHistoryExportStatusResponse({ success: true, data: { status: 'done', error_message: '' } }),
    { status: 'done', error_message: '' },
  );
  assert.throws(() => parseQueryHistoryExportStatusResponse({ success: true, data: { status: 'finished', error_message: '' } }), /status/);
  assert.throws(() => parseQueryHistoryExportStatusResponse({ success: true, data: { status: 'done' } }), /error_message/);
});

test('parseQueryHistoryShareTokenResponse requires a non-empty token', () => {
  assert.deepEqual(parseQueryHistoryShareTokenResponse({ success: true, data: { share_token: 'tok_1' } }), { share_token: 'tok_1' });
  assert.throws(() => parseQueryHistoryShareTokenResponse({ success: true, data: { share_token: '' } }), /share_token/);
  assert.throws(() => parseQueryHistoryShareTokenResponse({ success: true, data: {} }), /share_token/);
});

test('parseQueryHistoryConfigResponse narrows the mode union', () => {
  assert.deepEqual(parseQueryHistoryConfigResponse({ success: true, data: { mode: 'normal' } }), { mode: 'normal' });
  assert.deepEqual(parseQueryHistoryConfigResponse({ success: true, data: { mode: 'anonymized' } }), { mode: 'anonymized' });
  assert.deepEqual(parseQueryHistoryConfigResponse({ success: true, data: { mode: 'disabled' } }), { mode: 'disabled' });
  assert.throws(() => parseQueryHistoryConfigResponse({ success: true, data: { mode: 'off' } }), /mode/);
  assert.throws(() => parseQueryHistoryConfigResponse({ success: true, data: {} }), /mode/);
});

test('parse rejects non-envelope payloads', () => {
  assert.throws(() => parseQueryHistorySnapshotResponse({ data: snapshotEnvelope.data }), /success/);
  assert.throws(() => parseSharedSessionResponse(null), /expected an object envelope/);
  assert.throws(() => parseQueryHistoryExportStatusResponse({ success: false, data: {} }), /success/);
});
