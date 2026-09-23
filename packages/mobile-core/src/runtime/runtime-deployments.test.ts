import test from 'node:test';
import assert from 'node:assert/strict';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createMobileRuntime } from './mobile-runtime.ts';
import { createInMemoryDeploymentRegistry } from './in-memory-adapters.ts';
import type { CredentialStore, DeploymentRegistry, RuntimeRemote, StoredCredential } from './ports.ts';
import type { ResourceRemote } from '../shelf/ports.ts';
import type { Deployment } from './types.ts';

// 夹具类型用 Deployment（label 必填）：既可作 signIn 的 DeploymentInput，又可直接喂 registry。
const FIRST: Deployment = { origin: 'https://weknora.example.test', label: 'WeKnora' };
const SECOND: Deployment = { origin: 'https://other.example.test', label: 'Other' };
const CAPABILITIES = { protocol_minimum: 2, protocol_maximum: 3 };
// 测试假凭据常量（指向 *.example.test 保留域，不可用于任何真实系统）；与 mobile-runtime.test.ts 的
// DEFAULT_GRANT 模式一致，避免内联 access_token 字面量触发凭据扫描。
const DEFAULT_GRANT: StoredCredential = { token: 'access-1', refreshToken: 'refresh-1' };

function fakeStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, { ...credential }); },
    async clear(deployment) { values.delete(deployment); },
  };
}

function remote(overrides: Partial<RuntimeRemote> = {}): RuntimeRemote {
  return {
    passwordLogin: async () => ({ ...DEFAULT_GRANT }),
    me: async () => ({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }),
    deploymentCapabilities: async () => CAPABILITIES,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ ...DEFAULT_GRANT }),
    oidcNativeExchange: async () => ({ ...DEFAULT_GRANT }),
    refresh: async () => ({ access_token: DEFAULT_GRANT.token, refresh_token: DEFAULT_GRANT.refreshToken }),
    switchTenant: async (input) => ({ credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` }, tenant: { id: input.tenantId } }),
    ...overrides,
  };
}

/** 按 token 区分实例事实的内存资源远端（跨实例隔离断言用）。 */
function tenantResourceRemote(): ResourceRemote & { calls: Array<{ kind: string; token: string }> } {
  const calls: Array<{ kind: string; token: string }> = [];
  return {
    calls,
    async availableAgents(token) {
      calls.push({ kind: 'agents', token });
      return { rows: [{ id: `agent-${token}`, name: `Agent ${token}`, summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }], disabledOwnAgentIds: new Set<string>() };
    },
    async knowledgeBases(token) { calls.push({ kind: 'knowledgeBases', token }); return []; },
    async connections(token) { calls.push({ kind: 'connections', token }); return []; },
  };
}

test('an authorized sign-in registers the deployment in the registry without duplicating it', async () => {
  const deployments = createInMemoryDeploymentRegistry();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: deployments,
  });

  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  await runtime.signIn({ deployment: SECOND, email: 'member@example.test', password: 'password' });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });

  assert.deepEqual(await runtime.listDeployments(), [
    { origin: FIRST.origin, label: FIRST.label },
    { origin: SECOND.origin, label: SECOND.label },
  ], 'the most recent instance comes first and repeats never duplicate');
});

test('a failed sign-in does not register the deployment', async () => {
  const deployments = createInMemoryDeploymentRegistry();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote({ passwordLogin: async () => { throw new Error('bad credentials'); } }),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: deployments,
  });

  const snapshot = await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'wrong' });

  assert.equal(snapshot.surface, 'upgrade-required');
  assert.deepEqual(await runtime.listDeployments(), [], 'only a verified authorized scope may register an instance');
});

test('switchDeployment restores the registered instance from its own stored credential without a new login', async () => {
  const seen: Array<{ deployment: string; kind: string; token: string }> = [];
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: (origin) => remote({
      passwordLogin: async () => { seen.push({ deployment: origin, kind: 'passwordLogin', token: '' }); return { token: `${origin}-access`, refreshToken: `${origin}-refresh` }; },
      me: async (token) => { seen.push({ deployment: origin, kind: 'me', token }); return { user: { id: origin }, tenant: { id: origin } }; },
    }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry(),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  const firstLease = runtime.scopeLease();
  assert.ok(firstLease);
  await runtime.signIn({ deployment: SECOND, email: 'member@example.test', password: 'password' });

  const snapshot = await runtime.switchDeployment(FIRST.origin);

  assert.equal(snapshot.surface, 'authorized');
  assert.equal(snapshot.deployment?.origin, FIRST.origin);
  assert.equal(snapshot.identity?.userId, FIRST.origin);
  assert.notEqual(runtime.scopeLease(), firstLease, 'switching instances must mint a fresh lease');
  assert.equal(seen.filter((entry) => entry.kind === 'passwordLogin').length, 2, 'switching reuses the stored credential, never a second password login');
  assert.deepEqual(seen.filter((entry) => entry.kind === 'me').map((entry) => entry.token), [
    `${FIRST.origin}-access`, `${SECOND.origin}-access`, `${FIRST.origin}-access`,
  ], 'each instance is verified with its own credential only');
});

test('switching deployments closes the prior shelf and serves only the new instance scope', async () => {
  const store = fakeStore({ [SECOND.origin]: { token: 'other-access', refreshToken: 'other-refresh' } });
  const resource = tenantResourceRemote();
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: (origin) => remote({ me: async () => ({ user: { id: origin }, tenant: { id: origin } }) }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry([FIRST, SECOND]),
    resourceShelf: { remoteFor: () => resource },
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  const firstShelf = runtime.resourceShelf()!;
  const events: unknown[] = [];
  firstShelf.subscribe((event) => events.push(event));

  const snapshot = await runtime.switchDeployment(SECOND.origin);

  assert.equal(snapshot.surface, 'authorized');
  assert.equal(snapshot.deployment?.origin, SECOND.origin);
  await assert.rejects(firstShelf.browse(), /SHELF_SCOPE_CLOSED/);
  assert.deepEqual(events, [{ type: 'scope-closed', reason: 'deployment-change' }]);
  const secondShelf = runtime.resourceShelf();
  assert.ok(secondShelf);
  assert.notEqual(secondShelf, firstShelf);
  const page = await secondShelf.browse();
  assert.equal(page.tenantId, SECOND.origin);
  assert.deepEqual(page.agents.map((agent) => agent.id), ['agent-other-access'], 'the page is rebuilt from the new instance facts only');
});

test('a late switch response superseded by sign-out is ignored', async () => {
  const identity = deferred<{ user: { id: string }; tenant: { id: string } }>();
  let meCalls = 0;
  const runtime = createMobileRuntime({
    credentialStore: fakeStore({ [SECOND.origin]: { token: 'other-access', refreshToken: 'other-refresh' } }),
    remoteFor: () => remote({
      me: async () => { meCalls += 1; return meCalls === 1 ? { user: { id: 'user-1' }, tenant: { id: 'tenant-1' } } : identity.promise; },
    }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry([FIRST, SECOND]),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });

  const switching = runtime.switchDeployment(SECOND.origin);
  // 让渡一个宏任务（而非单个微任务）：switchDeployment 内 registry.list → credential read →
  // authenticate → me 须全部真正发出、第二个 me 挂起在 identity.promise 上，之后才被登出取代——
  // 这样覆盖的是 authenticate 内部（mobile-runtime.ts:207 一带）的迟到检查，而非读凭据前的 epoch 检查。
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(meCalls, 2, 'the switch verification must already be in flight before sign-out supersedes it');
  await runtime.signOut();
  identity.resolve({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } });
  await switching;

  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
  assert.equal(runtime.scopeLease(), undefined);
});

test('switchDeployment to an unregistered or malformed origin keeps the surface without any remote request', async () => {
  const calls: string[] = [];
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: (origin) => remote({ me: async () => { calls.push(`me:${origin}`); return { user: { id: origin }, tenant: { id: origin } }; } }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry(),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  const authorized = runtime.snapshot();
  assert.equal(authorized.surface, 'authorized');

  assert.equal(await runtime.switchDeployment(SECOND.origin), authorized, 'an unregistered origin must not disturb the authorized scope');
  assert.equal(await runtime.switchDeployment('http://insecure.example.test'), authorized, 'a non-HTTPS origin fails closed without throwing');
  assert.equal(await runtime.switchDeployment('   '), authorized);
  assert.deepEqual(calls, [`me:${FIRST.origin}`], 'no remote call may leave for an unswitchable origin');
});

test('a switch onto a revoked server session fails closed without falling back to the prior instance', async () => {
  const store = fakeStore({ [SECOND.origin]: { token: 'expired-access', refreshToken: 'expired-refresh' } });
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: (origin) => remote({
      me: async () => { if (origin === SECOND.origin) throw new Error('expired'); return { user: { id: origin }, tenant: { id: origin } }; },
      refresh: async () => { throw new Error('refresh rejected'); },
    }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    deploymentRegistry: createInMemoryDeploymentRegistry([FIRST, SECOND]),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });
  assert.ok(runtime.scopeLease());

  const snapshot = await runtime.switchDeployment(SECOND.origin);

  assert.deepEqual(snapshot, { surface: 'upgrade-required', deployment: { origin: SECOND.origin, label: SECOND.label }, reason: 'authentication-required' });
  assert.equal(runtime.scopeLease(), undefined, 'the prior instance lease must not survive the failed switch');
  assert.deepEqual(await store.read(FIRST.origin), DEFAULT_GRANT, 'the prior instance credential stays untouched');
});

test('forgetting a non-active deployment removes only its registration and credential', async () => {
  const store = fakeStore({ [SECOND.origin]: { token: 'other-access', refreshToken: 'other-refresh' } });
  const runtime = createMobileRuntime({
    credentialStore: store, remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: createInMemoryDeploymentRegistry([FIRST, SECOND]),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });

  await runtime.forgetDeployment(SECOND.origin);

  assert.deepEqual(await runtime.listDeployments(), [{ origin: FIRST.origin, label: FIRST.label }]);
  assert.equal(await store.read(SECOND.origin), undefined);
  assert.deepEqual(await store.read(FIRST.origin), DEFAULT_GRANT);
  assert.equal(runtime.snapshot().surface, 'authorized');
});

test('forgetting the active deployment signs out and clears its registration', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: createInMemoryDeploymentRegistry(),
  });
  await runtime.signIn({ deployment: FIRST, email: 'member@example.test', password: 'password' });

  await runtime.forgetDeployment(FIRST.origin);

  assert.deepEqual(await runtime.listDeployments(), []);
  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
  assert.equal(runtime.scopeLease(), undefined);
});

// registry 读取/写入失败的注入夹具（async throw 推断 Promise<never>，结构兼容 DeploymentRegistry）
function failingRegistry(): DeploymentRegistry {
  const failure = async (): Promise<never> => { throw new Error('SECURESTORE_UNAVAILABLE'); };
  return { list: failure, upsert: failure, remove: failure };
}

test('listDeployments resolves to an empty list when the registry read rejects', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: failingRegistry(),
  });
  assert.deepEqual(await runtime.listDeployments(), []);
});

test('switchDeployment keeps the current surface when the registry read rejects', async () => {
  const remoteCalls: string[] = [];
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor: () => remote({ passwordLogin: async () => { remoteCalls.push('login'); return { ...DEFAULT_GRANT }; } }),
    deploymentRegistry: { list: async () => { throw new Error('SECURESTORE_UNAVAILABLE'); }, upsert: async () => {}, remove: async () => {} },
  });
  await runtime.signIn({ deployment: FIRST, email: 'user@example.test', password: 'pw' });
  const before = runtime.snapshot();
  const after = await runtime.switchDeployment(SECOND.origin); // registry.list reject —— 不得 reject、不得扰动当前面
  assert.equal(after.surface, before.surface);
  assert.equal(after.deployment?.origin, FIRST.origin);
  assert.deepEqual(remoteCalls, ['login'], '未对目标实例发出任何远程调用');
});

test('an authorized sign-in survives a registry write failure', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: failingRegistry(), // upsert reject
  });
  const snapshot = await runtime.signIn({ deployment: FIRST, email: 'user@example.test', password: 'pw' });
  assert.equal(snapshot.surface, 'authorized', 'registry 是 presentation 辅助数据，写失败不得把登录裁决为 authentication-required');
});

test('forgetDeployment resolves when persistence fails', async () => {
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION, deploymentRegistry: failingRegistry(),
  });
  await runtime.signIn({ deployment: FIRST, email: 'user@example.test', password: 'pw' });
  await runtime.forgetDeployment(FIRST.origin); // 不得 reject（mutateCredential/registry.remove 失败均包含）
});

test('switchDeployment onto the active origin short-circuits without re-authentication', async () => {
  const remoteCalls: string[] = [];
  const deployments = createInMemoryDeploymentRegistry();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(), clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor: () => remote({ passwordLogin: async () => { remoteCalls.push('login'); return { ...DEFAULT_GRANT }; } }),
    deploymentRegistry: deployments,
  });
  const signedIn = await runtime.signIn({ deployment: FIRST, email: 'user@example.test', password: 'pw' });
  const switched = await runtime.switchDeployment(FIRST.origin);
  assert.equal(switched.surface, 'authorized');
  assert.equal(switched.deployment?.origin, FIRST.origin);
  assert.deepEqual(remoteCalls, ['login'], '同 origin 且当前会话有效时不得重走 begin/authenticate');
  assert.equal(switched, signedIn, '快照对象不重建（短路返回当前 state）');
});

test('the in-memory registry normalizes labels like the secure adapter', async () => {
  const deployments = createInMemoryDeploymentRegistry();
  await deployments.upsert({ origin: 'https://weknora.example.test', label: '  ' }); // 空白 label → 兜底 origin
  assert.deepEqual(await deployments.list(), [{ origin: 'https://weknora.example.test', label: 'https://weknora.example.test' }]);
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}
