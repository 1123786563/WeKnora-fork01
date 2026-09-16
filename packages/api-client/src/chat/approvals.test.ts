import assert from 'node:assert/strict';
import test from 'node:test';

import { ContractError } from '@weknora/contracts';

test('resolves tool approvals with an encoded pending id and exact decision payload', async () => {
  const { createChatApprovalsApi } = await import('./approvals.ts');
  const requests: unknown[] = [];
  const api = createChatApprovalsApi(async (request) => {
    requests.push(request);
    return { success: true };
  });

  await api.resolveTool('pending/1', {
    decision: 'approve',
    modifiedArgs: { path: '/tmp/safe' },
    reason: 'reviewed',
  });

  assert.deepEqual(requests, [{
    method: 'POST',
    path: '/api/v1/agent/tool-approvals/pending%2F1',
    body: { decision: 'approve', modified_args: { path: '/tmp/safe' }, reason: 'reviewed' },
  }]);
});

test('rejects invalid approval decisions and malformed success envelopes', async () => {
  const { createChatApprovalsApi } = await import('./approvals.ts');
  let calls = 0;
  const api = createChatApprovalsApi(async () => {
    calls += 1;
    return { success: false };
  });

  await assert.rejects(() => api.resolveTool('pending-1', { decision: 'allow' as 'approve' }), /approve or reject/);
  assert.equal(calls, 0);
  await assert.rejects(() => api.resolveTool('pending-1', { decision: 'reject' }), ContractError);
});

test('forwards the expected revision for approval CAS', async () => {
  const { createChatApprovalsApi } = await import('./approvals.ts');
  const requests: unknown[] = [];
  const api = createChatApprovalsApi(async (request) => { requests.push(request); return { success: true }; });
  await api.resolveTool('pending-1', { decision: 'reject', expected_revision: 7 });
  assert.deepEqual((requests[0] as { body: unknown }).body, { decision: 'reject', expected_revision: 7 });
});

test('resolves and cancels MCP OAuth through their distinct routes', async () => {
  const { createChatApprovalsApi } = await import('./approvals.ts');
  const requests: unknown[] = [];
  const api = createChatApprovalsApi(async (request) => {
    requests.push(request);
    return { success: true };
  });

  await api.resolveOAuth('oauth/1', { serviceId: 'service-1', decision: 'authorize' });
  await api.cancelOAuth('oauth/1');

  assert.deepEqual(requests, [
    {
      method: 'POST',
      path: '/api/v1/agent/mcp-oauth-resolutions/oauth%2F1',
      body: { service_id: 'service-1', decision: 'authorize' },
    },
    {
      method: 'POST',
      path: '/api/v1/agent/mcp-oauth-resolutions/oauth%2F1/cancel',
      body: {},
    },
  ]);
});
