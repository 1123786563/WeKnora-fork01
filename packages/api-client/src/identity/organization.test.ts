import assert from 'node:assert/strict';
import test from 'node:test';
import { createOrganizationApi } from './organization.ts';

test('maps organization list/search and member role routes with encoded ids', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createOrganizationApi(async (request) => {
    requests.push(request);
    if (request.path === '/api/v1/organizations') {
      return { success: true, data: { organizations: [{ id: 'org/1', name: 'Org', description: '', owner_id: 'u', owner_tenant_id: 7, created_at: 'now', updated_at: 'now' }], total: 1 } };
    }
    if (request.path === '/api/v1/organizations/search?q=acme&limit=5') {
      return { success: true, data: [{ id: 'org/1', name: 'Acme', description: '', member_count: 1, member_limit: 10, share_count: 0, is_already_member: false, require_approval: true }], total: 1 };
    }
    return { success: true };
  });

  assert.deepEqual(await api.list(), { items: [{ id: 'org/1', name: 'Org', description: '', owner_id: 'u', owner_tenant_id: 7, created_at: 'now', updated_at: 'now', has_pending_upgrade: false }], total: 1 });
  assert.deepEqual(await api.search('acme', 5), { items: [{ id: 'org/1', name: 'Acme', description: '', member_count: 1, member_limit: 10, share_count: 0, is_already_member: false, require_approval: true }], total: 1 });
  await api.members.updateRole('org/1', 7, { role: 'editor' });
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/organizations' },
    { method: 'GET', path: '/api/v1/organizations/search?q=acme&limit=5' },
    { method: 'PUT', path: '/api/v1/organizations/org%2F1/members/7', body: { role: 'editor' } },
  ]);
});

test('preserves organization sharing payloads and the server confirmation boundary', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createOrganizationApi(async (request) => {
    requests.push(request);
    return request.method === 'POST' && request.path.endsWith('/leave')
      ? { success: true, data: undefined }
      : { success: true, data: { id: 'share/1' } };
  });

  const share = await api.knowledgeBaseShares.create('kb/1', { organization_id: 'org/1', permission: 'viewer' });
  assert.deepEqual(share, { id: 'share/1' });
  await api.leave('org/1');
  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/knowledge-bases/kb%2F1/shares', body: { organization_id: 'org/1', permission: 'viewer' } },
    { method: 'POST', path: '/api/v1/organizations/org%2F1/leave', body: {} },
  ]);
});

test('lists organization shared resources and removes them through encoded owner routes', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createOrganizationApi(async (request) => {
    requests.push(request);
    if (request.path === '/api/v1/organizations/org%2F1/shares') {
      return { success: true, data: { shares: [{ id: 'kb-share-1', knowledge_base_id: 'kb/1', knowledge_base_name: 'Support' }], total: 1 } };
    }
    if (request.path === '/api/v1/organizations/org%2F1/agent-shares') {
      return { success: true, data: { shares: [{ id: 'agent-share-1', agent_id: 'agent/1', agent_name: 'Support agent' }], total: 1 } };
    }
    return { success: true };
  });

  assert.deepEqual(await api.knowledgeBaseShares.listForOrganization('org/1'), {
    items: [{ id: 'kb-share-1', knowledge_base_id: 'kb/1', knowledge_base_name: 'Support' }],
    total: 1,
  });
  assert.deepEqual(await api.agentShares.listForOrganization('org/1'), {
    items: [{ id: 'agent-share-1', agent_id: 'agent/1', agent_name: 'Support agent' }],
    total: 1,
  });
  await api.knowledgeBaseShares.remove('kb/1', 'kb-share-1');
  await api.agentShares.remove('agent/1', 'agent-share-1');
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/organizations/org%2F1/shares' },
    { method: 'GET', path: '/api/v1/organizations/org%2F1/agent-shares' },
    { method: 'DELETE', path: '/api/v1/knowledge-bases/kb%2F1/shares/kb-share-1' },
    { method: 'DELETE', path: '/api/v1/agents/agent%2F1/shares/agent-share-1' },
  ]);
});

test('exposes has_pending_upgrade from organization payloads (Vue modal parity)', async () => {
  const api = createOrganizationApi(async (request) => {
    if (request.path === '/api/v1/organizations/org%2F1') {
      return { success: true, data: { id: 'org/1', name: 'Org', description: '', owner_id: 'u', owner_tenant_id: 1, my_role: 'viewer', has_pending_upgrade: true, created_at: 'now', updated_at: 'now' } };
    }
    return { success: true, data: { id: 'org/2', name: 'Org2', description: '', owner_id: 'u', owner_tenant_id: 1, created_at: 'now', updated_at: 'now' } };
  });

  // Vue OrganizationSettingsModal.vue:1326 reads res.data.has_pending_upgrade
  // from the org detail endpoint (GET /organizations/:id).
  const detail = await api.get('org/1');
  assert.equal(detail.has_pending_upgrade, true);

  // A payload without the flag normalizes to false (Vue: `|| false`).
  const created = await api.create({ name: 'Org2' });
  assert.equal(created.has_pending_upgrade, false);
});
