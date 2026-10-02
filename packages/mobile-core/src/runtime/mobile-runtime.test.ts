import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createMobileRuntime } from './mobile-runtime.ts';
import type { CredentialStore, DeploymentStore, MobileRuntimePorts, PendingOidc, PendingOidcStore, RuntimeRemote, StoredCredential } from './ports.ts';
import type { DeploymentInput } from './types.ts';

const DEPLOYMENT: DeploymentInput = { origin: 'https://weknora.example.test', label: 'Test Deployment' };
const FULL_CAPABILITIES = { protocol_minimum: 2, protocol_maximum: 3 };
/** Fixture grants for the `.test` deployment; wire-shaped refresh responses are mapped from these. */
const DEFAULT_GRANT: StoredCredential = { token: 'access-1', refreshToken: 'refresh-1' };
const FRESH_GRANT: StoredCredential = { token: 'fresh-access', refreshToken: 'fresh-refresh' };
const LATE_GRANT: StoredCredential = { token: 'late-access', refreshToken: 'late-refresh' };

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function fakeStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore & { calls: string[] } {
  const values = new Map(Object.entries(initial));
  const calls: string[] = [];
  return {
    calls,
    async read(deployment) { calls.push(`read:${deployment}`); return values.get(deployment); },
    async write(deployment, credential) { calls.push(`write:${deployment}`); values.set(deployment, credential); },
    async clear(deployment) { calls.push(`clear:${deployment}`); values.delete(deployment); },
  };
}

function deploymentStore(initial?: DeploymentInput): DeploymentStore & { calls: string[] } {
  let value = initial && { ...initial };
  const calls: string[] = [];
  return {
    calls,
    async read() { calls.push('read'); return value && { ...value }; },
    async write(deployment) { calls.push(`write:${deployment.origin}`); value = { ...deployment }; },
    async clear() { calls.push('clear'); value = undefined; },
  };
}

function remote(overrides: Partial<RuntimeRemote> = {}): RuntimeRemote {
  return {
    passwordLogin: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    me: async () => ({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }),
    deploymentCapabilities: async () => FULL_CAPABILITIES,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    oidcNativeExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    refresh: async () => ({ access_token: DEFAULT_GRANT.token, refresh_token: DEFAULT_GRANT.refreshToken }),
    switchTenant: async (input) => ({ credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` }, tenant: { id: `tenant-${input.tenantId}` } }),
    ...overrides,
  };
}

function ports(store: CredentialStore = fakeStore(), remoteFor = (_deployment: string) => remote()): MobileRuntimePorts {
  return { credentialStore: store, remoteFor, clientVersion: CLIENT_PROTOCOL_VERSION };
}

function pendingStore(): PendingOidcStore & { value?: PendingOidc; calls: string[] } {
  const store: PendingOidcStore & { value?: PendingOidc; calls: string[] } = {
    calls: [],
    async savePending(input) { store.calls.push('save'); store.value = { ...input }; },
    async loadPending() { store.calls.push('load'); return store.value && { ...store.value }; },
    async consumePending() { store.calls.push('consume'); const pending = store.value; store.value = undefined; return pending && { ...pending }; },
    async clearPending() { store.calls.push('clear'); store.value = undefined; },
  };
  return store;
}

function delayedCredentialStore(): CredentialStore & { value?: StoredCredential; writeStarted: Promise<void>; releaseWrite(): void } {
  const writeStarted = deferred<void>();
  const writeReleased = deferred<void>();
  let value: StoredCredential | undefined;
  return {
    get value() { return value && { ...value }; },
    writeStarted: writeStarted.promise,
    releaseWrite: () => writeReleased.resolve(),
    async read() { return value && { ...value }; },
    async write(_deployment, credential) { writeStarted.resolve(); await writeReleased.promise; value = { ...credential }; },
    async clear() { value = undefined; },
  };
}

test('boot restores Task 2 credentials before identity and capabilities', async () => {
  const order: string[] = [];
  const store = fakeStore({ [DEPLOYMENT.origin]: { token: 'stored-access', refreshToken: 'stored-refresh' } });
  const runtime = createMobileRuntime(ports(store, () => remote({
    me: async (token) => { order.push(`me:${token}`); return { user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }; },
    deploymentCapabilities: async (token) => { order.push(`capabilities:${token}`); return FULL_CAPABILITIES; },
  })));

  await runtime.boot(DEPLOYMENT);

  assert.deepEqual(store.calls, [`read:${DEPLOYMENT.origin}`]);
  assert.deepEqual(order, ['me:stored-access', 'capabilities:stored-access']);
  assert.deepEqual(runtime.snapshot(), { surface: 'authorized', deployment: DEPLOYMENT, identity: { userId: 'user-1', activeTenantId: 'tenant-1', tenants: [{ id: 'tenant-1' }] } });
});

test('boot restores the persisted deployment and verifies stored credentials before authorizing', async () => {
  const order: string[] = [];
  const store = fakeStore({ [DEPLOYMENT.origin]: { token: 'stored-access', refreshToken: 'stored-refresh' } });
  const deployments = deploymentStore(DEPLOYMENT);
  const runtime = createMobileRuntime({
    ...ports(store, () => remote({
      me: async () => { order.push('me'); return { user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }; },
      deploymentCapabilities: async () => { order.push('capabilities'); return FULL_CAPABILITIES; },
    })),
    deploymentStore: deployments,
  });

  await runtime.boot();

  assert.deepEqual(deployments.calls, ['read', `write:${DEPLOYMENT.origin}`]);
  assert.deepEqual(store.calls, [`read:${DEPLOYMENT.origin}`]);
  assert.deepEqual(order, ['me', 'capabilities']);
  assert.equal(runtime.snapshot().surface, 'authorized');
});

test('authorized sign-in persists its deployment and sign-out clears it', async () => {
  const deployments = deploymentStore();
  const runtime = createMobileRuntime({ ...ports(), deploymentStore: deployments });

  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  await runtime.signOut();

  assert.deepEqual(deployments.calls, [`write:${DEPLOYMENT.origin}`, 'clear']);
});

test('a repeated boot hides the authorized snapshot synchronously while deployment restore is pending', async () => {
  const restored = deferred<{ origin: string; label?: string } | undefined>();
  const deployments: DeploymentStore = {
    read: async () => restored.promise,
    write: async () => {},
    clear: async () => {},
  };
  const runtime = createMobileRuntime({ ...ports(), deploymentStore: deployments });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.equal(runtime.snapshot().surface, 'authorized');
  assert.ok(runtime.scopeLease());

  const boot = runtime.boot();

  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
  assert.equal(runtime.scopeLease(), undefined);
  restored.resolve(DEPLOYMENT);
  await boot;
  assert.equal(runtime.snapshot().surface, 'authorized');
});

test('a delayed boot cannot override a later manual sign-in', async () => {
  const restored = deferred<{ origin: string; label?: string } | undefined>();
  const original = DEPLOYMENT;
  const manual: DeploymentInput = { origin: 'https://manual.example.test', label: 'Manual' };
  const store = fakeStore({ [original.origin]: { token: 'stored-access', refreshToken: 'stored-refresh' } });
  const deployments: DeploymentStore = {
    read: async () => restored.promise,
    write: async () => {},
    clear: async () => {},
  };
  const runtime = createMobileRuntime({
    ...ports(store, (origin) => remote({
      passwordLogin: async () => ({ token: `login-${origin}`, refreshToken: 'refresh' }),
      me: async () => ({ user: { id: origin }, tenant: { id: origin } }),
    })),
    deploymentStore: deployments,
  });

  const boot = runtime.boot();
  await Promise.resolve();
  await runtime.signIn({ deployment: manual, email: 'member@example.test', password: 'password' });
  restored.resolve(original);
  await boot;

  assert.deepEqual(runtime.snapshot(), {
    surface: 'authorized', deployment: manual,
    identity: { userId: manual.origin, activeTenantId: manual.origin, tenants: [{ id: manual.origin }] },
  });
});

test('sign-out clears deployment persistence after an in-flight authorized write', async () => {
  const writeStarted = deferred<void>();
  const finishWrite = deferred<void>();
  let stored: { origin: string; label?: string } | undefined;
  const deployments: DeploymentStore = {
    read: async () => stored,
    async write(deployment) { writeStarted.resolve(); await finishWrite.promise; stored = { ...deployment }; },
    async clear() { stored = undefined; },
  };
  const runtime = createMobileRuntime({ ...ports(), deploymentStore: deployments });

  const signIn = runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  await writeStarted.promise;
  const signOut = runtime.signOut();
  finishWrite.resolve();
  await Promise.all([signIn, signOut]);

  assert.equal(stored, undefined);
  assert.equal(runtime.snapshot().surface, 'deployment-login');
});

test('Task 2 AuthSession token reaches /auth/me and capabilities bearer requests', async () => {
  const seen: Array<{ path: string; authorization?: string }> = [];
  const task2Remote = createMobileRuntimeRemote({
    origin: DEPLOYMENT.origin,
    request: async (request) => {
      seen.push({ path: request.path, authorization: (request.headers as Record<string, string> | undefined)?.authorization });
      if (request.path === '/api/v1/auth/login') return { success: true, token: 'task-2-token', refresh_token: 'task-2-refresh' };
      if (request.path === '/api/v1/auth/me') return { success: true, data: { user: { id: 'user-1' }, tenant: { id: 7 } } };
      if (request.path === '/api/v1/system/capabilities') return { code: 0, data: FULL_CAPABILITIES };
      throw new Error(`unexpected request: ${request.path}`);
    },
  });
  const runtime = createMobileRuntime(ports(fakeStore(), () => task2Remote));

  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  assert.deepEqual(seen, [
    { path: '/api/v1/auth/login', authorization: undefined },
    { path: '/api/v1/auth/me', authorization: 'Bearer task-2-token' },
    { path: '/api/v1/system/capabilities', authorization: 'Bearer task-2-token' },
  ]);
  assert.equal(runtime.snapshot().surface, 'authorized');
});

test('only a full compatibility mode grants the authorized surface', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({ deploymentCapabilities: async () => ({ protocol_minimum: 4, protocol_maximum: 4 }) })));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.deepEqual(runtime.snapshot(), { surface: 'upgrade-required', deployment: DEPLOYMENT, reason: 'protocol-mismatch' });
  assert.equal(runtime.scopeLease(), undefined);
});

test('malformed and unknown capabilities remain on the upgrade-required surface', async () => {
  for (const capability of [{ protocol_minimum: '2', protocol_maximum: 3 }, { version: 'future' }]) {
    const runtime = createMobileRuntime(ports(fakeStore(), () => remote({ deploymentCapabilities: async () => capability })));
    await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
    assert.deepEqual(runtime.snapshot(), { surface: 'upgrade-required', deployment: DEPLOYMENT, reason: 'unknown-capability' });
  }
});

test('a missing active tenant stays safe', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({ me: async () => ({ user: { id: 'user-1' }, tenant: null }) })));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.deepEqual(runtime.snapshot(), { surface: 'upgrade-required', deployment: DEPLOYMENT, reason: 'tenant-required' });
  assert.equal(runtime.scopeLease(), undefined);
});

test('a deployment change clears the prior opaque lease and uses a new origin credential', async () => {
  const otherDeployment: DeploymentInput = { origin: 'https://other.example.test', label: 'Other' };
  const seen: Array<{ deployment: string; token: string }> = [];
  const runtime = createMobileRuntime(ports(fakeStore(), (deployment) => remote({
    passwordLogin: async () => ({ token: deployment === DEPLOYMENT.origin ? 'first-token' : 'second-token', refreshToken: 'refresh-1' }),
    me: async (token) => { seen.push({ deployment, token }); return { user: { id: deployment }, tenant: { id: deployment } }; },
  })));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const oldLease = runtime.scopeLease();
  assert.ok(oldLease);
  assert.equal('scopeLease' in runtime.snapshot(), false);
  await runtime.signIn({ deployment: otherDeployment, email: 'member@example.test', password: 'password' });
  assert.notEqual(runtime.scopeLease(), oldLease);
  assert.deepEqual(seen, [
    { deployment: DEPLOYMENT.origin, token: 'first-token' },
    { deployment: otherDeployment.origin, token: 'second-token' },
  ]);
});

test('late responses after sign out are ignored', async () => {
  const identity = deferred<{ user: { id: string }; tenant: { id: string } }>();
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({ me: async () => identity.promise })));
  const signIn = runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  await Promise.resolve();
  await runtime.signOut();
  identity.resolve({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } });
  await signIn;
  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
  assert.equal(runtime.scopeLease(), undefined);
});

test('sign-out clears a password credential whose write was already in flight', async () => {
  const store = delayedCredentialStore();
  const runtime = createMobileRuntime(ports(store));

  const signIn = runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  await store.writeStarted;
  const signOut = runtime.signOut();
  store.releaseWrite();
  await Promise.all([signIn, signOut]);

  assert.equal(store.value, undefined);
  assert.equal((await createMobileRuntime(ports(store)).boot(DEPLOYMENT)).surface, 'deployment-login');
});

test('sign-out clears an OIDC credential whose write was already in flight', async () => {
  const store = delayedCredentialStore();
  const pending = pendingStore();
  pending.value = { deploymentOrigin: DEPLOYMENT.origin, state: 'state-1', codeVerifier: 'verifier-1', redirectUri: 'weknora://oidc' };
  const runtime = createMobileRuntime({
    ...ports(store, () => remote({ oidcNativeExchange: async () => ({ token: 'oidc-access', refreshToken: 'oidc-refresh' }) })),
    pendingOidcStore: pending,
  });

  const completion = runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');
  await store.writeStarted;
  const signOut = runtime.signOut();
  store.releaseWrite();
  await Promise.all([completion, signOut]);

  assert.equal(store.value, undefined);
  assert.equal((await createMobileRuntime(ports(store)).boot(DEPLOYMENT)).surface, 'deployment-login');
});

test('sign-out clears pending OIDC so a later callback cannot exchange or authorize', async () => {
  const pending = pendingStore();
  pending.value = { deploymentOrigin: DEPLOYMENT.origin, state: 'state-1', codeVerifier: 'verifier-1', redirectUri: 'weknora://oidc' };
  let exchanges = 0;
  const store = fakeStore();
  const runtime = createMobileRuntime({
    ...ports(store, () => remote({ oidcNativeExchange: async () => { exchanges += 1; return { token: 'oidc-access', refreshToken: 'oidc-refresh' }; } })),
    pendingOidcStore: pending,
  });

  await runtime.signOut();
  await runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');

  assert.equal(exchanges, 0);
  assert.deepEqual(store.calls, []);
  assert.equal(pending.value, undefined);
  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
});

test('sign-out clears a pending OIDC save that finishes after the logout epoch', async () => {
  const saveStarted = deferred<void>();
  const releaseSave = deferred<void>();
  let value: PendingOidc | undefined;
  const pending: PendingOidcStore = {
    async savePending(input) { saveStarted.resolve(); await releaseSave.promise; value = { ...input }; },
    async loadPending() { return value && { ...value }; },
    async consumePending() { const claimed = value; value = undefined; return claimed && { ...claimed }; },
    async clearPending() { value = undefined; },
  };
  const runtime = createMobileRuntime({
    ...ports(), pendingOidcStore: pending,
    oidcBrowser: { async open() { return ''; } },
    randomBytes: (size) => new Uint8Array(size).fill(7),
  });

  const begin = runtime.beginOidc({ deployment: DEPLOYMENT, redirectUri: 'weknora://oidc' });
  await saveStarted.promise;
  const signOut = runtime.signOut();
  releaseSave.resolve();
  await Promise.all([begin, signOut]);

  assert.equal(value, undefined);
});

test('an expired credential refreshes once, persists the rotation, and continues identity verification', async () => {
  const store = fakeStore({ [DEPLOYMENT.origin]: { token: 'expired-access', refreshToken: 'refresh-1' } });
  const seen: string[] = [];
  const runtime = createMobileRuntime(ports(store, () => remote({
    me: async (token) => {
      seen.push(`me:${token}`);
      if (token === 'expired-access') throw new Error('expired');
      return { user: { id: 'user-1' }, tenant: { id: 'tenant-1' } };
    },
    refresh: async (refreshToken) => { seen.push(`refresh:${refreshToken}`); return { access_token: 'rotated-access', refresh_token: 'rotated-refresh' }; },
    deploymentCapabilities: async (token) => { seen.push(`capabilities:${token}`); return FULL_CAPABILITIES; },
  })));

  const snapshot = await runtime.boot(DEPLOYMENT);

  assert.equal(snapshot.surface, 'authorized');
  assert.deepEqual(seen, ['me:expired-access', 'refresh:refresh-1', 'me:rotated-access', 'capabilities:rotated-access']);
  assert.deepEqual(await store.read(DEPLOYMENT.origin), { token: 'rotated-access', refreshToken: 'rotated-refresh' });
});

test('concurrent OIDC completions share one refresh after identity rejects an expired token', async () => {
  const pending = pendingStore();
  pending.value = { deploymentOrigin: DEPLOYMENT.origin, state: 'state-1', codeVerifier: 'verifier-1', redirectUri: 'weknora://oidc' };
  const refreshStarted = deferred<void>();
  const releaseRefresh = deferred<{ access_token: string; refresh_token: string }>();
  let refreshes = 0;
  const runtime = createMobileRuntime({
    ...ports(fakeStore(), () => remote({
      oidcNativeExchange: async () => ({ token: 'expired-access', refreshToken: 'refresh-1' }),
      me: async (token) => { if (token === 'expired-access') throw new Error('expired'); return { user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }; },
      refresh: async () => { refreshes += 1; refreshStarted.resolve(); return releaseRefresh.promise; },
    })),
    pendingOidcStore: pending,
  });

  const first = runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');
  const second = runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');
  await refreshStarted.promise;
  assert.equal(refreshes, 1);
  releaseRefresh.resolve({ access_token: FRESH_GRANT.token, refresh_token: FRESH_GRANT.refreshToken });
  await Promise.all([first, second]);

  assert.equal(refreshes, 1);
  assert.equal(runtime.snapshot().surface, 'authorized');
});

test('a refresh that settles after sign-out cannot persist or authorize', async () => {
  const store = fakeStore({ [DEPLOYMENT.origin]: { token: 'expired-access', refreshToken: 'refresh-1' } });
  const refreshStarted = deferred<void>();
  const releaseRefresh = deferred<{ access_token: string; refresh_token: string }>();
  const runtime = createMobileRuntime(ports(store, () => remote({
    me: async () => { throw new Error('expired'); },
    refresh: async () => { refreshStarted.resolve(); return releaseRefresh.promise; },
  })));

  const boot = runtime.boot(DEPLOYMENT);
  await refreshStarted.promise;
  await runtime.signOut();
  releaseRefresh.resolve({ access_token: LATE_GRANT.token, refresh_token: LATE_GRANT.refreshToken });
  await boot;

  assert.equal(await store.read(DEPLOYMENT.origin), undefined);
  assert.deepEqual(runtime.snapshot(), { surface: 'deployment-login', reason: 'authentication-required' });
});

test('a refresh that settles after a deployment change cannot persist or authorize the prior deployment', async () => {
  const other: DeploymentInput = { origin: 'https://other.example.test', label: 'Other' };
  const store = fakeStore({ [DEPLOYMENT.origin]: { token: 'expired-access', refreshToken: 'refresh-1' } });
  const refreshStarted = deferred<void>();
  const releaseRefresh = deferred<{ access_token: string; refresh_token: string }>();
  const runtime = createMobileRuntime(ports(store, (origin) => remote({
    passwordLogin: async () => ({ token: 'other-access', refreshToken: 'other-refresh' }),
    me: async (token) => {
      if (origin === DEPLOYMENT.origin && token === 'expired-access') throw new Error('expired');
      return { user: { id: 'other-user' }, tenant: { id: 'other-tenant' } };
    },
    refresh: async () => { refreshStarted.resolve(); return releaseRefresh.promise; },
  })));

  const boot = runtime.boot(DEPLOYMENT);
  await refreshStarted.promise;
  await runtime.signIn({ deployment: other, email: 'member@example.test', password: 'password' });
  releaseRefresh.resolve({ access_token: LATE_GRANT.token, refresh_token: LATE_GRANT.refreshToken });
  await boot;

  assert.deepEqual(await store.read(DEPLOYMENT.origin), { token: 'expired-access', refreshToken: 'refresh-1' });
  assert.deepEqual(runtime.snapshot(), {
    surface: 'authorized', deployment: other, identity: { userId: 'other-user', activeTenantId: 'other-tenant', tenants: [{ id: 'other-tenant' }] },
  });
});

test('a failed refresh remains on the fail-closed authentication surface', async () => {
  const store = fakeStore({ [DEPLOYMENT.origin]: { token: 'expired-access', refreshToken: 'refresh-1' } });
  const runtime = createMobileRuntime(ports(store, () => remote({
    me: async () => { throw new Error('expired'); },
    refresh: async () => { throw new Error('refresh rejected'); },
  })));

  const snapshot = await runtime.boot(DEPLOYMENT);

  assert.deepEqual(snapshot, { surface: 'upgrade-required', deployment: DEPLOYMENT, reason: 'authentication-required' });
  assert.deepEqual(await store.read(DEPLOYMENT.origin), { token: 'expired-access', refreshToken: 'refresh-1' });
});

test('OIDC persists the verifier and server state before browser launch then a fresh Runtime consumes them once without duplicate delivery revoking authorization', async () => {
  const pending = pendingStore();
  const browserCalls: string[] = [];
  const exchangeCalls: Array<{ code: string; state: string; redirectUri: string; codeVerifier: string }> = [];
  const oidcRemote = remote({
    oidcUrl: async (_serverRedirect, frontendRedirect, challenge) => {
      assert.equal(frontendRedirect, 'weknora://oidc');
      assert.equal(challenge, '3Ev4DHdHPRMPoN6GukAY_pi7IUAF5qWJHRK6kURvnoE');
      return { authorizationUrl: 'https://idp.example.test/authorize', state: 'server-state' };
    },
    oidcExchange: async () => { throw new Error('provider-code endpoint must not receive native handoff'); },
    oidcNativeExchange: async (input) => {
      exchangeCalls.push(input);
      return { token: 'oidc-access', refreshToken: 'oidc-refresh' };
    },
  });
  const store = fakeStore();
  const start = createMobileRuntime({
    ...ports(store, () => oidcRemote), pendingOidcStore: pending,
    oidcBrowser: { async open(url) { browserCalls.push(url); return 'weknora://oidc?code=code-1&state=server-state'; } },
    randomBytes: (size) => new Uint8Array(size).fill(7),
  });

  await start.beginOidc({ deployment: { origin: 'https://weknora.example.test/' }, redirectUri: 'weknora://oidc' });

  assert.deepEqual(browserCalls, ['https://idp.example.test/authorize']);
  assert.deepEqual(pending.calls, ['save']);
  assert.deepEqual(pending.value && { ...pending.value, codeVerifier: pending.value.codeVerifier.length }, {
    deploymentOrigin: DEPLOYMENT.origin, state: 'server-state', redirectUri: 'weknora://oidc', codeVerifier: 43,
  });
  const persistedVerifier = pending.value!.codeVerifier;

  const resumed = createMobileRuntime({ ...ports(store, () => oidcRemote), pendingOidcStore: pending });
  await resumed.completeOidc('weknora://oidc?code=code-1&state=server-state');

  assert.deepEqual(exchangeCalls, [{ code: 'code-1', state: 'server-state', redirectUri: 'weknora://oidc', codeVerifier: persistedVerifier }]);
  assert.deepEqual(store.calls, [`write:${DEPLOYMENT.origin}`]);
  assert.equal(resumed.snapshot().surface, 'authorized');
  const authorizedSnapshot = resumed.snapshot();
  const authorizedLease = resumed.scopeLease();
  await resumed.completeOidc('weknora://oidc?code=code-1&state=server-state');
  assert.equal(exchangeCalls.length, 1);
  assert.deepEqual(pending.calls, ['save', 'consume']);
  assert.equal(resumed.snapshot(), authorizedSnapshot);
  assert.equal(resumed.scopeLease(), authorizedLease);
});

test('concurrent native callbacks claim one persisted handoff and exchange it once', async () => {
  const pending = pendingStore();
  pending.value = { deploymentOrigin: DEPLOYMENT.origin, state: 'state-1', codeVerifier: 'verifier-1', redirectUri: 'weknora://oidc' };
  let exchanges = 0;
  const exchangeStarted = deferred<void>();
  const release = deferred<StoredCredential>();
  const oidcRemote = remote({
    oidcExchange: async () => { throw new Error('provider-code endpoint must not receive native handoff'); },
    oidcNativeExchange: async () => { exchanges += 1; exchangeStarted.resolve(); return release.promise; },
  });
  const runtime = createMobileRuntime({ ...ports(fakeStore(), () => oidcRemote), pendingOidcStore: pending });

  const first = runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');
  const second = runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');
  await exchangeStarted.promise;
  assert.equal(exchanges, 1);
  release.resolve({ token: 'access-1', refreshToken: 'refresh-1' });
  await Promise.all([first, second]);

  assert.equal(exchanges, 1);
  assert.deepEqual(pending.calls, ['consume']);
});

test('OIDC consumes and rejects a callback outside its exact registered redirect', async () => {
  const pending = pendingStore();
  pending.value = { deploymentOrigin: DEPLOYMENT.origin, state: 'state-1', codeVerifier: 'verifier-1', redirectUri: 'weknora://oidc' };
  const runtime = createMobileRuntime({ ...ports(), pendingOidcStore: pending });

  await runtime.completeOidc('weknora://other?code=code-1&state=state-1');

  assert.deepEqual(pending.calls, ['consume']);
  assert.equal(pending.value, undefined);
  assert.deepEqual(runtime.snapshot(), { surface: 'upgrade-required', deployment: { origin: DEPLOYMENT.origin, label: DEPLOYMENT.origin }, reason: 'authentication-required' });
});

test('activateTenant re-issues the credential, publishes the new active tenant, and revokes the prior lease', async () => {
  const store = fakeStore();
  const switches: Array<{ tenantId: string; refreshToken: string }> = [];
  const runtime = createMobileRuntime(ports(store, () => remote({
    me: async (token) => ({ user: { id: 'user-1' }, tenant: { id: token === 'tenant-2-access' ? 'tenant-2' : 'tenant-1' } }),
    switchTenant: async (input) => {
      switches.push(input);
      return { credential: { token: 'tenant-2-access', refreshToken: 'rotated-refresh' }, tenant: { id: 'tenant-2' } };
    },
  })));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const priorLease = runtime.scopeLease();
  assert.ok(priorLease);

  const snapshot = await runtime.activateTenant('2');

  assert.deepEqual(switches, [{ tenantId: '2', refreshToken: 'refresh-1', accessToken: 'access-1' }]);
  assert.equal(snapshot.identity?.activeTenantId, 'tenant-2');
  assert.notEqual(runtime.scopeLease(), priorLease);
  assert.equal(runtime.snapshot().identity?.activeTenantId, 'tenant-2');
  assert.deepEqual(await store.read(DEPLOYMENT.origin), { token: 'tenant-2-access', refreshToken: 'rotated-refresh' });
});

test('the authorized snapshot lists tenant options parsed from memberships', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({
    me: async () => ({
      user: { id: 'user-1' }, tenant: { id: 7 },
      memberships: [{ tenant_id: 7, tenant_name: 'Acme', role: 'owner' }, { tenant_id: 9, tenant_name: 'Beta', role: 'viewer' }],
    }),
  })));
  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.deepEqual(snapshot.identity, {
    userId: 'user-1', activeTenantId: '7',
    tenants: [{ id: '7', name: 'Acme' }, { id: '9', name: 'Beta' }],
  });
});

test('a snapshot without memberships still exposes the active tenant as the sole option', async () => {
  const runtime = createMobileRuntime(ports(fakeStore()));
  const snapshot = await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  assert.deepEqual(snapshot.identity, { userId: 'user-1', activeTenantId: 'tenant-1', tenants: [{ id: 'tenant-1' }] });
});

test('activateTenant without an authorized surface issues no switch request', async () => {
  let switches = 0;
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({
    switchTenant: async () => { switches += 1; return { credential: { token: 'x', refreshToken: 'y' }, tenant: { id: 't' } }; },
  })));
  const before = runtime.snapshot();

  const snapshot = await runtime.activateTenant('2');
  await runtime.activateTenant('');
  await runtime.activateTenant('   ');

  assert.equal(switches, 0);
  assert.equal(snapshot, before);
});

test('a rejected tenant switch fails closed without keeping the prior lease', async () => {
  const store = fakeStore();
  const runtime = createMobileRuntime(ports(store, () => remote({
    switchTenant: async () => { throw new Error('no membership'); },
  })));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const priorLease = runtime.scopeLease();
  assert.ok(priorLease);

  const snapshot = await runtime.activateTenant('404');

  assert.deepEqual(snapshot, { surface: 'upgrade-required', deployment: DEPLOYMENT, reason: 'authentication-required' });
  assert.notEqual(runtime.scopeLease(), priorLease);
  assert.equal(runtime.scopeLease(), undefined);
  assert.deepEqual(await store.read(DEPLOYMENT.origin), { token: 'access-1', refreshToken: 'refresh-1' });
});

test('authorizedRequest rejects before any transport call when unauthorized', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote()));
  await assert.rejects(runtime.authorizedRequest({ method: 'GET', path: '/api/v1/workbench/overview' }), /RUNTIME_UNAUTHORIZED/);
});

test('authorizedRequest carries the credential and refreshes exactly once on a 401', async () => {
  const sentTokens: Array<string | undefined> = [];
  const rotatedCredential = { token: 'access-2', refreshToken: 'refresh-2' };
  const store = fakeStore();
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: () => remote({
      me: async (token) => ({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }),
      refresh: async () => ({ access_token: rotatedCredential.token, refresh_token: rotatedCredential.refreshToken }),
    }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedTransport: () => async (input, accessToken) => {
      sentTokens.push(`${input.method} ${input.path} ${accessToken}`);
      if (accessToken === 'access-1') {
        const error = new Error('HTTP 401');
        error.name = 'ApiError';
        (error as unknown as { status?: number }).status = 401;
        throw error;
      }
      return { success: true, data: { ok: true } };
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const response = await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/workbench/overview' });
  assert.deepEqual(response, { success: true, data: { ok: true } });
  assert.deepEqual(sentTokens, [
    'GET /api/v1/workbench/overview access-1',
    'GET /api/v1/workbench/overview access-2',
  ]);
});

test('authorizedRequest does not refresh or replay a POST after a 401', async () => {
  const store = fakeStore();
  let sends = 0;
  let refreshes = 0;
  const unauthorized = Object.assign(new Error('HTTP 401'), { name: 'ApiError', status: 401 });
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: () => remote({ refresh: async () => { refreshes += 1; return { access_token: 'access-2', refresh_token: 'refresh-2' }; } }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedTransport: () => async () => { sends += 1; throw unauthorized; },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  await assert.rejects(runtime.authorizedRequest({ method: 'POST', path: '/api/v1/tasks', body: { title: 'task' } }), (error) => error === unauthorized);

  assert.equal(sends, 1);
  assert.equal(refreshes, 0);
});

test('authorizedRequest returns scope changed instead of a stale write 401 after sign-out', async () => {
  const started = deferred<void>();
  const response = deferred<void>();
  const unauthorized = Object.assign(new Error('HTTP 401'), { name: 'ApiError', status: 401 });
  const runtime = createMobileRuntime({
    ...ports(fakeStore(), () => remote()),
    authorizedTransport: () => async () => { started.resolve(); await response.promise; throw unauthorized; },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  const pending = runtime.authorizedRequest({ method: 'POST', path: '/api/v1/tasks', body: { title: 'task' } });
  await started.promise;
  await runtime.signOut();
  response.resolve();

  await assert.rejects(pending, /RUNTIME_SCOPE_CHANGED/);
});

test('authorizedRequest refreshes and replays HEAD and OPTIONS after a 401', async () => {
  for (const method of ['HEAD', 'OPTIONS']) {
    let refreshes = 0;
    const sentTokens: string[] = [];
    const runtime = createMobileRuntime({
      ...ports(fakeStore(), () => remote({ refresh: async () => { refreshes += 1; return { access_token: 'access-2', refresh_token: 'refresh-2' }; } })),
      authorizedTransport: () => async (input, token) => {
        sentTokens.push(`${input.method}:${token}`);
        if (token === 'access-1') throw Object.assign(new Error('HTTP 401'), { name: 'ApiError', status: 401 });
        return { success: true, data: { ok: true } };
      },
    });
    await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

    assert.deepEqual(await runtime.authorizedRequest({ method, path: '/api/v1/tasks' }), { success: true, data: { ok: true } });
    assert.deepEqual(sentTokens, [`${method}:access-1`, `${method}:access-2`]);
    assert.equal(refreshes, 1);
  }
});

test('authorizedRequest does not retry PUT, PATCH, or DELETE and normalizes read methods', async () => {
  let sends = 0;
  let refreshes = 0;
  const unauthorized = Object.assign(new Error('HTTP 401'), { name: 'ApiError', status: 401 });
  const runtime = createMobileRuntime({
    ...ports(fakeStore(), () => remote({ refresh: async () => { refreshes += 1; return { access_token: 'access-2', refresh_token: 'refresh-2' }; } })),
    authorizedTransport: () => async () => { sends += 1; throw unauthorized; },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  for (const method of ['PUT', 'PATCH', 'DELETE']) {
    await assert.rejects(runtime.authorizedRequest({ method, path: '/api/v1/tasks/1' }), (error) => error === unauthorized);
  }
  await assert.rejects(runtime.authorizedRequest({ method: 'get', path: '/api/v1/tasks' }), (error) => error === unauthorized);

  assert.equal(sends, 5, 'three writes send once and lowercase GET retries once');
  assert.equal(refreshes, 1, 'lowercase GET refreshes exactly once');
});

test('an expired Task Office session with failed refresh rejects the protected request and exposes no data', async () => {
  const store = fakeStore();
  let sends = 0;
  let refreshes = 0;
  const runtime = createMobileRuntime({
    credentialStore: store,
    remoteFor: () => remote({ refresh: async () => { refreshes += 1; throw new Error('refresh expired'); } }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedTransport: () => async () => {
      sends += 1;
      const error = new Error('HTTP 401');
      error.name = 'ApiError';
      (error as unknown as { status?: number }).status = 401;
      throw error;
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  await assert.rejects(runtime.authorizedRequest({ method: 'GET', path: '/api/v1/tasks' }), /RUNTIME_UNAUTHORIZED/);
  assert.equal(refreshes, 1, 'expired credentials trigger one refresh attempt');
  assert.equal(sends, 1, 'a failed refresh never replays the Task request');
  runtime.dispose();
});

test('a late authorized response after a scope change is dropped', async () => {
  const release = deferred<void>();
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedTransport: () => async () => {
      await release.promise;
      return { success: true, data: { stale: true } };
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const pending = runtime.authorizedRequest({ method: 'GET', path: '/api/v1/workbench/overview' });
  await runtime.signOut();
  release.resolve();
  await assert.rejects(pending, /RUNTIME_SCOPE_CHANGED/);
});

test('a late tenant verification cannot override a completed later switch', async () => {
  const store = fakeStore();
  const meStarted = deferred<void>();
  const releaseMe = deferred<{ user: { id: string }; tenant: { id: string } }>();
  let meCalls = 0;
  const runtime = createMobileRuntime(ports(store, () => remote({
    me: async (token) => {
      meCalls += 1;
      if (meCalls === 2) { meStarted.resolve(); return releaseMe.promise; }
      const tenantId = token === 'tenant-2-access' ? 'tenant-2' : token === 'tenant-3-access' ? 'tenant-3' : 'tenant-1';
      return { user: { id: 'user-1' }, tenant: { id: tenantId } };
    },
  })));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });

  const first = runtime.activateTenant('2');
  await meStarted.promise;
  const second = await runtime.activateTenant('3');
  releaseMe.resolve({ user: { id: 'user-1' }, tenant: { id: 'tenant-2' } });
  const firstSnapshot = await first;

  assert.equal(second.identity?.activeTenantId, 'tenant-3');
  assert.equal(runtime.snapshot().identity?.activeTenantId, 'tenant-3');
  assert.equal(firstSnapshot.identity?.activeTenantId, 'tenant-3');
});

test('authorizedEventStream rejects before any transport call when unauthorized', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote()));
  await assert.rejects(runtime.authorizedEventStream({ method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2' }, () => {}), /RUNTIME_UNAUTHORIZED/);
});

test('authorizedEventStream delivers chunks with the active token and refreshes exactly once on a pre-stream 401', async () => {
  const sentTokens: string[] = [];
  const chunks: string[] = [];
  const rotated = { token: 'access-2', refreshToken: 'refresh-2' };
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote({ refresh: async () => ({ access_token: rotated.token, refresh_token: rotated.refreshToken }) }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedStream: () => async (_input, accessToken, onChunk) => {
      sentTokens.push(accessToken);
      if (accessToken === 'access-1') {
        const error = new Error('HTTP 401');
        error.name = 'ApiError';
        (error as unknown as { status?: number }).status = 401;
        throw error; // pre-stream 401：未产出任何 chunk
      }
      onChunk('id: 3\nevent: run.started\n');
      onChunk('data: {}\n\n');
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  await runtime.authorizedEventStream({ method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2' }, (chunk) => chunks.push(chunk));
  assert.deepEqual(sentTokens, ['access-1', 'access-2']);
  assert.equal(chunks.join(''), 'id: 3\nevent: run.started\ndata: {}\n\n');
});

test('a stream opened before a scope change is dropped, and its chunks never flush', async () => {
  const release = deferred<void>();
  const chunks: string[] = [];
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedStream: () => async (_input, _accessToken, onChunk) => {
      await release.promise;
      onChunk('late frame');
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const pending = runtime.authorizedEventStream({ method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2' }, (chunk) => chunks.push(chunk));
  await runtime.signOut();
  release.resolve();
  await assert.rejects(pending, /RUNTIME_SCOPE_CHANGED/);
  assert.deepEqual(chunks, [], 'no late frame is delivered after the scope died');
});

test('a stream still open when the scope dies stops delivering chunks and settles with RUNTIME_SCOPE_CHANGED', async () => {
  const streamOpened = deferred<void>();
  const release = deferred<void>();
  const chunks: string[] = [];
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedStream: () => async (_input, _accessToken, onChunk) => {
      streamOpened.resolve();
      onChunk('id: 1\nevent: run.started\n\n');
      await release.promise;
      onChunk('late frame after scope death');
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const pending = runtime.authorizedEventStream({ method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2' }, (chunk) => chunks.push(chunk));
  await streamOpened.promise;
  await runtime.signOut();
  release.resolve();
  await assert.rejects(pending, /RUNTIME_SCOPE_CHANGED/);
  assert.deepEqual(chunks, ['id: 1\nevent: run.started\n\n'], 'pre-death frames were delivered; the post-death frame never flushes');
});

test('authorizedEventStream distinguishes an unavailable stream channel from unauthorized', async () => {
  // 无 authorizedStream 端口（ports() 夹具不提供）：面已授权，仅流通道不可用
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote()));
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  await assert.rejects(
    runtime.authorizedEventStream({ method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2' }, () => {}),
    /RUNTIME_STREAM_UNAVAILABLE/,
  );
});

test('revoking the scope aborts an in-flight authorized stream without waiting for the next chunk (R1-F32)', async () => {
  let aborted = false;
  const streamPorts = ports(fakeStore(), () => remote());
  streamPorts.authorizedStream = () => async (input) => {
    await new Promise<void>((_resolve, reject) => {
      input.signal?.addEventListener('abort', () => { aborted = true; reject(new Error('aborted')); });
    });
  };
  const streaming = createMobileRuntime(streamPorts);
  await streaming.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const reading = streaming.authorizedEventStream({ method: 'GET', path: '/api/v1/events' }, () => {});
  const settled = reading.catch(() => undefined); // 先挂 catch：signOut 触发的 rejection 不得落入 unhandled
  await new Promise((resolve) => setImmediate(resolve));
  await streaming.signOut(); // revoke 路径（begin/reserve/dispose 共用）
  await settled;
  assert.ok(aborted, 'scope 撤销必须主动 abort 传输 signal，而不是等下一个 chunk 的守卫拒绝');
});
