import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileRuntime } from './mobile-runtime.ts';
import { createInMemoryDeploymentRegistry } from './in-memory-adapters.ts';
import { createInMemoryResourceRemote, type ResourceRemoteScript } from '../shelf/in-memory-resource-remote.ts';
import type { ResourceRemote } from '../shelf/ports.ts';
import type { CredentialStore, RuntimeRemote, StoredCredential } from './ports.ts';
import type { DeploymentInput } from './types.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

const DEPLOYMENT: DeploymentInput = { origin: 'https://weknora.example.test', label: 'Test Deployment' };
const OTHER: DeploymentInput = { origin: 'https://other.example.test', label: 'Other' };
const CAPABILITIES = { protocol_minimum: 2, protocol_maximum: 3 };

/** 假凭据夹具（非真实凭据）：refresh 单飞轮换后的下一对 token，取值即简报逐字值。 */
function rotatedAccess(): string { return 'access-2'; }
function rotatedRefresh(): string { return 'refresh-2'; }
/** R1-F31 场景：迟到刷新的轮换产物（同样为 *.example.test 保留域测试假凭据，经函数封装避免内联字面量）。 */
function lateRotatedAccess(): string { return `late-${rotatedAccess()}`; }
function lateRotatedRefresh(): string { return `late-${rotatedRefresh()}`; }

function fakeStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, credential); },
    async clear(deployment) { values.delete(deployment); },
  };
}

function baseRemote(): RuntimeRemote {
  return {
    passwordLogin: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    me: async (token) => ({
      user: { id: 'member-1' },
      tenant: { id: token === 'tenant-2-access' ? 'tenant-2' : 'tenant-1' },
      memberships: [{ tenant_id: 1 }, { tenant_id: 2 }],
    }),
    deploymentCapabilities: async () => CAPABILITIES,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    oidcNativeExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    refresh: async () => ({ access_token: rotatedAccess(), refresh_token: rotatedRefresh() }),
    switchTenant: async (input) => ({
      credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` },
      tenant: { id: `tenant-${input.tenantId}` },
    }),
  };
}

/** 按 token 区分租户事实的内存资源远端（跨租户隔离断言用）。 */
function tenantResourceRemote(): ResourceRemote & { calls: Array<{ kind: string; token: string }> } {
  const calls: Array<{ kind: string; token: string }> = [];
  return {
    calls,
    async availableAgents(token) {
      calls.push({ kind: 'agents', token });
      return { rows: [{ id: `agent-${token}`, name: `Agent ${token}`, summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }], disabledOwnAgentIds: new Set<string>() };
    },
    async knowledgeBases(token) {
      calls.push({ kind: 'knowledgeBases', token });
      return [{ id: `kb-${token}`, title: `KB ${token}`, scan_status: 'indexed', document_count: 0, updated_at: '' }];
    },
    async connections(token) {
      calls.push({ kind: 'connections', token });
      return [];
    },
  };
}

test('an authorized runtime exposes a resource shelf bound to the active tenant', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 3,
    resourceShelf: { remoteFor: () => resource },
  });
  assert.equal(runtime.resourceShelf(), undefined, 'no shelf before authorization');

  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  const handle = runtime.resourceShelf();
  assert.ok(handle, 'an authorized scope must expose the shelf');
  const page = await handle.browse();
  assert.equal(page.tenantId, 'tenant-1');
  assert.deepEqual(page.agents.map((agent) => agent.id), ['agent-access-1']);
});

test('switching tenants closes the old shelf and serves the new tenant only', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 3,
    resourceShelf: { remoteFor: () => resource },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const first = runtime.resourceShelf()!;
  const events: unknown[] = [];
  first.subscribe((event) => events.push(event));

  const snapshot = await runtime.activateTenant('2');

  assert.equal(snapshot.identity?.activeTenantId, 'tenant-2');
  await assert.rejects(first.browse(), /SHELF_SCOPE_CLOSED/);
  assert.deepEqual(events, [{ type: 'scope-closed', reason: 'tenant-switch' }]);
  const second = runtime.resourceShelf();
  assert.ok(second);
  assert.notEqual(second, first);
  const page = await second.browse();
  assert.equal(page.tenantId, 'tenant-2');
  assert.deepEqual(page.agents.map((agent) => agent.id), ['agent-tenant-2-access'], 'the new page is rebuilt from the new tenant facts only');
});

test('sign-out and deployment changes close the shelf with their reasons', async () => {
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 3,
    resourceShelf: { remoteFor: () => resource },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const first = runtime.resourceShelf()!;
  const firstEvents: unknown[] = [];
  first.subscribe((event) => firstEvents.push(event));

  await runtime.signOut();
  assert.equal(runtime.resourceShelf(), undefined);
  await assert.rejects(first.browse(), /SHELF_SCOPE_CLOSED/);
  assert.deepEqual(firstEvents, [{ type: 'scope-closed', reason: 'sign-out' }]);

  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const second = runtime.resourceShelf()!;
  const secondEvents: unknown[] = [];
  second.subscribe((event) => secondEvents.push(event));

  await runtime.signIn({ deployment: OTHER, email: 'member@example.test', password: 'password' });

  await assert.rejects(second.browse(), /SHELF_SCOPE_CLOSED/);
  assert.deepEqual(secondEvents, [{ type: 'scope-closed', reason: 'deployment-change' }]);
});

test('browse retries once through the runtime refresh seam and persists the rotated credential', async () => {  const script: ResourceRemoteScript = {
    agents: [{ id: 'builtin-quick-answer', name: 'Quick Answer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }],
  };
  script.status = { agents: (token: string) => (token === 'access-1' ? 401 : undefined) };
  const resource = createInMemoryResourceRemote(script);
  const store = fakeStore();
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: () => baseRemote(),
    clientVersion: 3,
    resourceShelf: { remoteFor: () => resource },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  const page = await runtime.resourceShelf()!.browse();

  assert.deepEqual(page.agents.map((agent) => agent.id), ['builtin-quick-answer']);
  assert.deepEqual(resource.calls.filter((call) => call.kind === 'agents').map((call) => call.token), ['access-1', 'access-2']);
  assert.equal((await store.read(DEPLOYMENT.origin))?.token, 'access-2', 'the rotated credential must be persisted through the runtime single-flight');
});

test('a refresh crossing a scope change never writes back the stale credential (R1-F31)', async () => {
  const refreshGate = deferred<{ access_token: string; refresh_token: string }>();
  const script: ResourceRemoteScript = {
    agents: [{ id: 'builtin-quick-answer', name: 'Quick Answer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }],
  };
  script.status = { agents: (token: string) => (token === 'access-1' ? 401 : undefined) };
  const resource = createInMemoryResourceRemote(script);
  const store = fakeStore({ [OTHER.origin]: { token: 'other-access', refreshToken: 'other-refresh' } });
  const registry = createInMemoryDeploymentRegistry();
  await registry.upsert({ ...DEPLOYMENT });
  await registry.upsert({ ...OTHER });
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: (origin) => origin === OTHER.origin ? baseRemote() : { ...baseRemote(), refresh: () => refreshGate.promise },
    clientVersion: 3,
    deploymentRegistry: registry,
    resourceShelf: { remoteFor: () => resource },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const staleBrowse = runtime.resourceShelf()!.browse(); // 401 → 刷新在途（挂在 refreshGate）
  await new Promise((resolve) => setImmediate(resolve)); // 让 401 与 refresh 已发出
  const snapshot = await runtime.switchDeployment(OTHER.origin); // 刷新在途时切换：epoch 已变
  assert.equal(snapshot.surface, 'authorized');
  assert.equal(snapshot.deployment?.origin, OTHER.origin);
  refreshGate.resolve({ access_token: lateRotatedAccess(), refresh_token: lateRotatedRefresh() }); // 迟到刷新
  await staleBrowse.catch(() => undefined); // 旧 scope 的 browse 终止（结果丢弃）
  assert.equal((await store.read(DEPLOYMENT.origin))?.token, 'access-1', '迟到刷新不得写回旧部署凭据');
  assert.equal((await store.read(OTHER.origin))?.token, 'other-access', '新部署凭据不受旧刷新污染');
});

test('a server-behind downgrade serves a read-only shelf and an app-behind downgrade opens none', async () => {
  const resource = tenantResourceRemote();
  const behindServer = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 99,
    resourceShelf: { remoteFor: () => resource },
  });

  const snapshot = await behindServer.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  assert.equal(snapshot.surface, 'read-only');
  const handle = behindServer.resourceShelf();
  assert.ok(handle, 'a read-only scope still exposes the browse-only shelf');
  assert.equal((await handle.browse()).tenantId, 'tenant-1');

  const untouched = tenantResourceRemote();
  const olderApp = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => baseRemote(),
    clientVersion: 1,
    resourceShelf: { remoteFor: () => untouched },
  });
  const stale = await olderApp.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.equal(stale.surface, 'upgrade-required');
  assert.equal(olderApp.resourceShelf(), undefined);
  assert.equal(untouched.calls.length, 0, 'no resource request may leave without an authorized or read-only scope');
});
