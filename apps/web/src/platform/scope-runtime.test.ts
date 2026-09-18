import assert from 'node:assert/strict';
import test from 'node:test';
import { createWebScopeRuntime } from './scope-runtime.ts';

test('hydrates auth/me into the active user, tenant, and capability snapshot', () => {
  const runtime = createWebScopeRuntime('https://api.test', null, null);
  const next = runtime.hydrate({
    user: { id: 'user-2' },
    tenant: { id: 7 },
    tenant_required: false,
    capabilities: {
      organizations: { supported: false, reason: 'lite' },
      agents: { supported: true },
    },
  });

  assert.deepEqual(next.scope, {
    origin: 'https://api.test',
    userId: 'user-2',
    tenantId: '7',
    generation: next.scope.generation,
  });
  assert.equal(runtime.can('organizations'), false);
  assert.equal(runtime.can('agents'), true);
});

test('auth/me with no active tenant clears the old tenant and exposes onboarding state', () => {
  const runtime = createWebScopeRuntime('https://api.test', 'user-1', 'tenant-a');
  runtime.hydrate({ user: { id: 'user-1' }, tenant: null, tenant_required: true, capabilities: {} });
  assert.equal(runtime.current().scope.tenantId, null);
  assert.equal(runtime.current().scope.userId, 'user-1');
  assert.equal(runtime.requiresWorkspace(), true);
});

test('hydrate preserves the same user\'s persisted tenant override over the home tenant', () => {
  const runtime = createWebScopeRuntime('https://api.test', 'user-1', 'tenant-2');

  runtime.hydrate({ user: { id: 'user-1' }, tenant: { id: 1 }, tenant_required: false });

  assert.equal(runtime.current().scope.tenantId, 'tenant-2');
});

test('exposes channel-session visibility only for active-tenant admins', () => {
  const runtime = createWebScopeRuntime('https://api.test', null, null);
  runtime.hydrate({ user: { id: 'user-2' }, tenant: { id: 7 }, memberships: [{ tenant_id: 7, role: 'viewer' }] });
  assert.equal(runtime.canViewChannelSessions(), false);
  runtime.hydrate({ user: { id: 'user-2' }, tenant: { id: 7 }, memberships: [{ tenant_id: 7, role: 'admin' }] });
  assert.equal(runtime.canViewChannelSessions(), true);
});

test('switchTenant commits the new scope only after auth.switchTenant returns a session', async () => {
  const runtime = createWebScopeRuntime('https://api.test', 'user-1', 'tenant-a');
  const previous = runtime.current();
  const persisted: string[] = [];
  const next = await runtime.switchTenant('7', async (tenantId, refreshToken) => {
    assert.equal(tenantId, 7);
    assert.equal(refreshToken, 'refresh-a');
    return { token: 'access-b', refreshToken: 'refresh-b', tenant: { id: 7 } };
  }, async (session) => {
    persisted.push(session.token);
  }, 'refresh-a');

  assert.equal(previous.signal.aborted, true);
  assert.equal(next.scope.tenantId, '7');
  assert.deepEqual(persisted, ['access-b']);
});

test('persists the active tenant whenever a scope change is committed', () => {
  const persisted: Array<string | null> = [];
  const runtime = createWebScopeRuntime('https://api.test', 'user-1', 'tenant-a', {
    persistTenant: (tenantId) => persisted.push(tenantId),
  });
  runtime.setTenant('tenant-b');
  runtime.logout();
  assert.deepEqual(persisted, ['tenant-b', null]);
});

test('tenant switches abort the previous scope and produce a scoped query key', () => {
  const runtime = createWebScopeRuntime('https://api.test', 'user-1', 'tenant-a');
  const previous = runtime.current();
  const next = runtime.setTenant('tenant-b');
  assert.equal(previous.signal.aborted, true);
  assert.equal(runtime.key('knowledge-bases')[3], 'tenant-b');
  assert.equal(runtime.controller.isCurrent(next.scope), true);
});

test('logout invalidates the active generation so late data cannot be accepted', () => {
  const runtime = createWebScopeRuntime('https://api.test', 'user-1', 'tenant-a');
  const previous = runtime.current();
  runtime.controller.logout();
  assert.equal(previous.signal.aborted, true);
  assert.equal(runtime.controller.isCurrent(previous.scope), false);
});

// R441 A4: hydrate is the /auth/me landing — it must mirror Vue setUser and
// write the weknora_user identity so per-user preference namespacing follows
// the active account. logout clears it.
test('hydrate writes weknora_user; logout clears it (Vue stores/auth.ts parity)', () => {
  const store = new Map<string, string>();
  const storage = {
    getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
  };
  const runtime = createWebScopeRuntime('https://api.test', null, null, { storage });

  runtime.hydrate({ user: { id: 'user-9', email: 'u9@test.dev', nickname: 'Nine' }, tenant: { id: 1 }, capabilities: {} });
  assert.deepEqual(JSON.parse(storage.getItem('weknora_user')!), { id: 'user-9', email: 'u9@test.dev', nickname: 'Nine' });

  runtime.logout();
  assert.equal(storage.getItem('weknora_user'), null);
});

test('hydrate without storage injected stays a no-op (embed / non-browser safe)', () => {
  const runtime = createWebScopeRuntime('https://api.test', null, null);
  assert.doesNotThrow(() => runtime.hydrate({ user: { id: 'user-1' }, tenant: null, capabilities: {} }));
  assert.equal(runtime.current().scope.userId, 'user-1');
});
