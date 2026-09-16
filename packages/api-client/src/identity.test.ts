import assert from 'node:assert/strict';
import test from 'node:test';
import { ApiError } from './errors.ts';
import { createIdentityApi } from './identity/index.ts';

test('maps tenant members, invitations, leave, and audit cursor without swallowing failures', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const identity = createIdentityApi(async (request) => {
    requests.push(request);
    if (request.path.startsWith('/api/v1/tenants/7/members?')) {
      return { success: true, data: { members: [{ user_id: 'u/1', email: 'a@example.test', username: 'a', role: 'owner', status: 'active', joined_at: 'now' }], total: 1, page: 2, page_size: 25 } };
    }
    if (request.path === '/api/v1/me/invitations/pending-count') return { success: true, data: { pending_count: 2 } };
    if (request.path === '/api/v1/tenants/7/audit-log?after_id=8&limit=10&outcome=denied') return { success: true, data: [], next_cursor: 0 };
    return { success: true };
  });

  assert.deepEqual(await identity.tenants.members.list(7, { q: 'a/b', page: 2, pageSize: 25 }), {
    items: [{ user_id: 'u/1', email: 'a@example.test', username: 'a', role: 'owner', status: 'active', joined_at: 'now' }],
    total: 1,
    page: 2,
    pageSize: 25,
  });
  assert.equal((await identity.invitations.pendingCount()).pendingCount, 2);
  assert.deepEqual(await identity.tenants.auditLog.list(7, { afterId: 8, limit: 10, outcome: 'denied' }), { items: [], nextCursor: 0 });
  await identity.tenants.members.leave(7);
  assert.deepEqual(requests.map((request) => [request.method, request.path]), [
    ['GET', '/api/v1/tenants/7/members?q=a%2Fb&page=2&page_size=25'],
    ['GET', '/api/v1/me/invitations/pending-count'],
    ['GET', '/api/v1/tenants/7/audit-log?after_id=8&limit=10&outcome=denied'],
    ['POST', '/api/v1/tenants/7/leave'],
  ]);
});

test('accepts audit rows whose optional scope and request metadata are empty', async () => {
  const identity = createIdentityApi(async () => ({
    success: true,
    data: [{
      id: 1,
      tenant_id: 7,
      actor_user_id: 'owner-1',
      actor_role: 'owner',
      action: 'rbac.invitation_sent',
      scope_type: '',
      scope_id: '',
      target_type: 'tenant_invitation',
      target_id: '1',
      target_user_id: 'invitee-1',
      request_path: '',
      request_method: '',
      outcome: 'success',
      details: { role: 'viewer' },
      created_at: 'now',
    }],
    next_cursor: 1,
  }));

  assert.deepEqual(await identity.tenants.auditLog.list(7), {
    items: [{
      id: 1,
      tenant_id: 7,
      actor_user_id: 'owner-1',
      actor_role: 'owner',
      action: 'rbac.invitation_sent',
      scope_type: '',
      scope_id: '',
      target_type: 'tenant_invitation',
      target_id: '1',
      target_user_id: 'invitee-1',
      request_path: '',
      request_method: '',
      outcome: 'success',
      details: { role: 'viewer' },
      created_at: 'now',
    }],
    nextCursor: 1,
  });
});

test('encodes tenant invitation ids and returns typed memberships', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const identity = createIdentityApi(async (request) => {
    requests.push(request);
    if (request.path === '/api/v1/me/invitations/12/accept') {
      return { success: true, data: { membership: { tenant_id: 7, role: 'viewer', status: 'active', joined_at: 'now' } } };
    }
    if (request.path === '/api/v1/me/invitations/accept-by-token') {
      return { success: true, data: { membership: { tenant_id: 7, role: 'viewer', status: 'active', joined_at: 'now' }, tenant_name: 'Tenant' } };
    }
    return { success: true };
  });

  assert.deepEqual(await identity.invitations.accept(12), {
    tenantId: 7,
    role: 'viewer',
    status: 'active',
    joinedAt: 'now',
  });
  await identity.invitations.acceptByToken('token/a');
  await identity.invitations.decline(12);
  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/me/invitations/12/accept' },
    { method: 'POST', path: '/api/v1/me/invitations/accept-by-token', body: { token: 'token/a' } },
    { method: 'POST', path: '/api/v1/me/invitations/12/decline' },
  ]);
});

test('does not convert a last-owner conflict into a successful mutation', async () => {
  const identity = createIdentityApi(async () => {
    throw new ApiError({ status: 409, code: 'last_owner', message: 'last owner cannot leave' });
  });
  await assert.rejects(() => identity.tenants.members.leave(7), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.status, 409);
    assert.equal(error.code, 'last_owner');
    return true;
  });
});
