import assert from 'node:assert/strict';
import test from 'node:test';

import { createQueryHistoryApi } from './index.ts';
import type { ClientBinaryResponse } from '../client.ts';

interface RecordedCall {
  method: string;
  path: string;
  body?: unknown;
}

function recordingRequest(calls: RecordedCall[], payload: () => unknown = () => ({ success: true, data: {} })) {
  return async (input: { method: string; path: string; body?: unknown }) => {
    calls.push({ method: input.method, path: input.path, ...(input.body === undefined ? {} : { body: input.body }) });
    return payload();
  };
}

test('queryHistory adminList maps every audit filter onto the sessions query', async () => {
  const calls: RecordedCall[] = [];
  const api = createQueryHistoryApi(recordingRequest(calls, () => ({
    success: true,
    data: [{ id: 's1', title: 't', is_pinned: false, user_id: 'u1', engine_type: 'builtin' }],
    total: 1,
    page: 0,
    page_size: 20,
  })));
  const result = await api.adminList({
    page: 0,
    pageSize: 20,
    keyword: '报表',
    source: 'all',
    agentId: 'agent-1',
    userId: 'u1',
    startTime: '2026-09-01',
    endTime: '2026-09-19',
    feedback: 'like',
  });
  assert.deepEqual(calls, [{
    method: 'GET',
    path: '/api/v1/sessions?page=0&page_size=20&keyword=%E6%8A%A5%E8%A1%A8&source=all&agent_id=agent-1&user_id=u1&start_time=2026-09-01&end_time=2026-09-19&feedback=like',
  }]);
  assert.equal(result.data[0]!.user_id, 'u1');
  assert.equal(result.total, 1);
});

test('queryHistory adminList omits the query string without params and validates paging/feedback', async () => {
  const calls: RecordedCall[] = [];
  const api = createQueryHistoryApi(recordingRequest(calls, () => ({
    success: true,
    data: [],
    total: 0,
    page: 0,
    page_size: 50,
  })));
  await api.adminList();
  assert.deepEqual(calls, [{ method: 'GET', path: '/api/v1/sessions' }]);
  await assert.rejects(api.adminList({ page: -1 }), /page must be a non-negative integer/);
  await assert.rejects(api.adminList({ pageSize: 0 }), /pageSize must be a positive integer/);
  await assert.rejects(api.adminList({ feedback: 'meh' as 'like' }), /feedback must be like or dislike/);
});

test('queryHistory snapshot hits the admin snapshot path', async () => {
  const calls: RecordedCall[] = [];
  const api = createQueryHistoryApi(recordingRequest(calls, () => ({
    success: true,
    data: {
      session: { id: 's1', title: 't', is_pinned: false },
      messages: [{ id: 'm1', session_id: 's1', role: 'user', content: 'hi' }],
      feedback: [],
      truncated: false,
    },
  })));
  const result = await api.snapshot('ses/1');
  assert.deepEqual(calls, [{ method: 'GET', path: '/api/v1/admin/sessions/ses%2F1/snapshot' }]);
  assert.equal(result.session.id, 's1');
  await assert.rejects(api.snapshot('  '), /sessionId must not be empty/);
});

test('queryHistory startExport posts the optional filter body', async () => {
  const calls: RecordedCall[] = [];
  const api = createQueryHistoryApi(recordingRequest(calls, () => ({ success: true, data: { job_id: 9 } })));
  const result = await api.startExport({ userId: 'u1', startTime: '2026-09-01', endTime: '2026-09-19', feedback: 'dislike' });
  assert.deepEqual(calls, [{
    method: 'POST',
    path: '/api/v1/admin/sessions/export',
    body: { user_id: 'u1', start_time: '2026-09-01', end_time: '2026-09-19', feedback: 'dislike' },
  }]);
  assert.deepEqual(result, { job_id: 9 });
  await api.startExport();
  assert.deepEqual(calls[1]!.body, {});
  await assert.rejects(api.startExport({ feedback: 'meh' as 'like' }), /feedback must be like or dislike/);
});

test('queryHistory exportStatus maps the job id onto the status path', async () => {
  const calls: RecordedCall[] = [];
  const api = createQueryHistoryApi(recordingRequest(calls, () => ({
    success: true,
    data: { job_id: 9, status: 'running', error_message: '' },
  })));
  const result = await api.exportStatus(9);
  assert.deepEqual(calls, [{ method: 'GET', path: '/api/v1/admin/sessions/export/9/status' }]);
  assert.equal(result.status, 'running');
  await api.exportStatus('abc');
  assert.equal(calls[1]!.path, '/api/v1/admin/sessions/export/abc/status');
  await assert.rejects(api.exportStatus(' '), /jobId must not be empty/);
});

test('queryHistory downloadExport rides the binary channel', async () => {
  const binaryCalls: RecordedCall[] = [];
  const csv: ClientBinaryResponse = {
    body: '\uFEFFsession_id,title',
    contentType: 'text/csv; charset=utf-8',
    headers: { 'content-disposition': 'attachment; filename=query_history_export_9.csv' },
  };
  const api = createQueryHistoryApi(
    recordingRequest([]),
    async (input) => {
      binaryCalls.push({ method: input.method, path: input.path });
      return csv;
    },
  );
  const result = await api.downloadExport(9);
  assert.deepEqual(binaryCalls, [{ method: 'GET', path: '/api/v1/admin/sessions/export/9/download' }]);
  assert.equal(result, csv);
});

test('queryHistory downloadExport fails fast without a binary channel', async () => {
  const api = createQueryHistoryApi(recordingRequest([]));
  await assert.rejects(api.downloadExport(9), /Binary transport is unavailable/);
});

test('queryHistory share/unshare hit the session share endpoints', async () => {
  const calls: RecordedCall[] = [];
  const api = createQueryHistoryApi(recordingRequest(calls, () => {
    if (calls[calls.length - 1]!.method === 'POST') return { success: true, data: { share_token: 'tok_1' } };
    return { success: true, data: {} };
  }));
  const token = await api.share('s1');
  assert.deepEqual(token, { share_token: 'tok_1' });
  await api.unshare('s1');
  assert.deepEqual(calls, [
    { method: 'POST', path: '/api/v1/sessions/s1/share' },
    { method: 'DELETE', path: '/api/v1/sessions/s1/share' },
  ]);
  await assert.rejects(api.share('  '), /sessionId must not be empty/);
});

test('queryHistory shared hits the public token path', async () => {
  const calls: RecordedCall[] = [];
  const api = createQueryHistoryApi(recordingRequest(calls, () => ({
    success: true,
    data: {
      session: { id: 's1', title: 't', is_pinned: false },
      messages: [],
      truncated: false,
    },
  })));
  const result = await api.shared('tok/1');
  assert.deepEqual(calls, [{ method: 'GET', path: '/api/v1/shared/sessions/tok%2F1' }]);
  assert.equal(result.truncated, false);
  await assert.rejects(api.shared('  '), /token must not be empty/);
});

test('queryHistory queryHistoryConfig reads and writes the tenant KV key', async () => {
  const calls: RecordedCall[] = [];
  const api = createQueryHistoryApi(recordingRequest(calls, () => ({ success: true, data: { mode: 'anonymized' } })));
  assert.deepEqual(await api.queryHistoryConfig.get(), { mode: 'anonymized' });
  assert.deepEqual(await api.queryHistoryConfig.update('disabled'), { mode: 'anonymized' });
  assert.deepEqual(calls, [
    { method: 'GET', path: '/api/v1/tenants/kv/query-history-config' },
    { method: 'PUT', path: '/api/v1/tenants/kv/query-history-config', body: { mode: 'disabled' } },
  ]);
  await assert.rejects(api.queryHistoryConfig.update('off' as 'normal'), /mode must be normal, anonymized, or disabled/);
});

test('queryHistory contract parsing rejects a bad snapshot envelope', async () => {
  const api = createQueryHistoryApi(recordingRequest([], () => ({ success: true, data: { session: null } })));
  await assert.rejects(api.snapshot('s1'), /expected an array/);
});
