import assert from 'node:assert/strict';
import test from 'node:test';

import { createUsageApi } from './index.ts';
import type { ClientBinaryResponse } from '../client.ts';

test('usage my maps params to start_time/end_time query', async () => {
  const calls: Array<{ method: string; path: string }> = [];
  const api = createUsageApi(async (input) => {
    calls.push({ method: input.method, path: input.path });
    return { success: true, data: [] };
  });
  const result = await api.my({ startTime: '2026-09-01', endTime: '2026-09-19' });
  assert.deepEqual(calls, [{ method: 'GET', path: '/api/v1/usage/me?start_time=2026-09-01&end_time=2026-09-19' }]);
  assert.deepEqual(result.items, []);
});

test('usage byUser maps page/pageSize and range to the admin query', async () => {
  const calls: Array<{ method: string; path: string }> = [];
  const api = createUsageApi(async (input) => {
    calls.push({ method: input.method, path: input.path });
    return { success: true, data: [] };
  });
  await api.byUser({ page: 0, pageSize: 50, startTime: '2026-09-01', endTime: '2026-09-19' });
  assert.deepEqual(calls, [{
    method: 'GET',
    path: '/api/v1/admin/usage/by-user?page=0&page_size=50&start_time=2026-09-01&end_time=2026-09-19',
  }]);
});

test('usage endpoints omit the query string without params', async () => {
  const paths: string[] = [];
  const api = createUsageApi(async (input) => {
    paths.push(input.path);
    return { success: true, data: [] };
  });
  await api.my();
  await api.byUser();
  assert.deepEqual(paths, ['/api/v1/usage/me', '/api/v1/admin/usage/by-user']);
});

test('usage byUser rejects out-of-range paging values', async () => {
  const api = createUsageApi(async () => ({ success: true, data: [] }));
  await assert.rejects(api.byUser({ page: -1 }), /page must be a non-negative integer/);
  await assert.rejects(api.byUser({ pageSize: 0 }), /pageSize must be a positive integer/);
});

test('usage exportCsv rides the binary channel and maps the range query', async () => {
  const binaryCalls: Array<{ method: string; path: string }> = [];
  const csv: ClientBinaryResponse = { body: '\uFEFFuser_id,model', contentType: 'text/csv; charset=utf-8', headers: { 'content-disposition': 'attachment; filename=usage_export.csv' } };
  const api = createUsageApi(
    async () => ({ success: true, data: [] }),
    async (input) => {
      binaryCalls.push({ method: input.method, path: input.path });
      return csv;
    },
  );
  const result = await api.exportCsv({ startTime: '2026-09-01' });
  assert.deepEqual(binaryCalls, [{ method: 'GET', path: '/api/v1/admin/usage/export?start_time=2026-09-01' }]);
  assert.equal(result, csv);
});

test('usage exportCsv fails fast without a binary channel', async () => {
  const api = createUsageApi(async () => ({ success: true, data: [] }));
  await assert.rejects(api.exportCsv(), /Binary transport is unavailable/);
});

test('usage parsing rejects a non-array data payload', async () => {
  const api = createUsageApi(async () => ({ success: true, data: {} }));
  await assert.rejects(api.my(), /expected an array/);
});
