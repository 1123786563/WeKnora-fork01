import assert from 'node:assert/strict';
import test from 'node:test';

import { createAnalyticsApi } from './index.ts';

test('analytics queryTrend maps params to start_time/end_time query', async () => {
  const calls: Array<{ method: string; path: string }> = [];
  const api = createAnalyticsApi(async (input) => {
    calls.push({ method: input.method, path: input.path });
    return { success: true, data: [] };
  });
  await api.queryTrend({ startTime: '2026-09-01', endTime: '2026-09-19' });
  assert.deepEqual(calls, [{ method: 'GET', path: '/api/v1/analytics/queries?start_time=2026-09-01&end_time=2026-09-19' }]);
});

test('agentUsage rejects empty agentId', async () => {
  const api = createAnalyticsApi(async () => ({ success: true, data: [] }));
  await assert.rejects(api.agentUsage('  '), /agentId must not be empty/);
});

test('analytics range endpoints omit the query string without params', async () => {
  const paths: string[] = [];
  const api = createAnalyticsApi(async (input) => {
    paths.push(input.path);
    return { success: true, data: [] };
  });
  await api.queryTrend();
  await api.activeUsers({ endTime: '2026-09-19' });
  await api.channelSessions({ startTime: '2026-09-01' });
  await api.agentUsage('agent/one');
  assert.deepEqual(paths, [
    '/api/v1/analytics/queries',
    '/api/v1/analytics/users?end_time=2026-09-19',
    '/api/v1/analytics/channels?start_time=2026-09-01',
    '/api/v1/analytics/agents/agent%2Fone',
  ]);
});

test('analytics parsing rejects a non-array data payload', async () => {
  const api = createAnalyticsApi(async () => ({ success: true, data: {} }));
  await assert.rejects(api.queryTrend(), /expected an array/);
});
