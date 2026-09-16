import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveProductSessionResources } from './resources.ts';

function session(rows: Record<string, Record<string, unknown>>) {
  return {
    baseURL: 'https://api.example',
    transport: { send: async (request: any) => ({ status: 200, headers: {}, body: { success: true, data: rows[new URL(request.url).pathname.split('/').pop()!] } }) },
  } as any;
}

test('queries every product resource and rejects cross tenant resources', async () => {
  const identity = { origin: 'https://api.example', userId: 'u1', tenantId: 't1' };
  const selection = { spaceId: 's1', agentId: 'a1', targetId: 't1', workspaceRef: 'w1' };
  const rows = {
    s1: { id: 's1', owner_tenant_id: 't1' },
    a1: { id: 'a1', name: 'Research', tenant_id: 't1' },
    t1: { id: 't1', tenant_id: 't1', owner_id: 'u1', state: 'active' },
    w1: { id: 'w1', target_id: 't1', tenant_id: 't1' },
  };
  assert.equal((await resolveProductSessionResources(session(rows), selection, identity, new AbortController().signal)).agentName, 'Research');
  await assert.rejects(() => resolveProductSessionResources(session({ ...rows, a1: { ...rows.a1, tenant_id: 'other' } }), selection, identity, new AbortController().signal), /tenant ownership/);
});
