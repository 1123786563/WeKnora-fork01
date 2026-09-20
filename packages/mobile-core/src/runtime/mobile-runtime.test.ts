import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileRuntime } from './mobile-runtime.ts';
import type { CredentialStore, MobileRuntimePorts, PendingOidc, PendingOidcStore, RuntimeRemote, StoredCredential } from './ports.ts';
import type { DeploymentInput } from './types.ts';

const DEPLOYMENT: DeploymentInput = { origin: 'https://weknora.example.test', label: 'Test Deployment' };
const FULL_CAPABILITIES = { protocol_minimum: 2, protocol_maximum: 3 };

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

function remote(overrides: Partial<RuntimeRemote> = {}): RuntimeRemote {
  return {
    passwordLogin: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    me: async () => ({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }),
    deploymentCapabilities: async () => FULL_CAPABILITIES,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    oidcNativeExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    ...overrides,
  };
}

function ports(store = fakeStore(), remoteFor = (_deployment: string) => remote()): MobileRuntimePorts {
  return { credentialStore: store, remoteFor, clientVersion: 3 };
}

function pendingStore(): PendingOidcStore & { value?: PendingOidc; calls: string[] } {
  const store: PendingOidcStore & { value?: PendingOidc; calls: string[] } = {
    calls: [],
    async savePending(input) { store.calls.push('save'); store.value = { ...input }; },
    async loadPending() { store.calls.push('load'); return store.value && { ...store.value }; },
    async consumePending() { store.calls.push('consume'); const pending = store.value; store.value = undefined; return pending && { ...pending }; },
  };
  return store;
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
  assert.deepEqual(runtime.snapshot(), { surface: 'authorized', deployment: DEPLOYMENT, identity: { userId: 'user-1', activeTenantId: 'tenant-1' } });
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

test('OIDC persists the verifier and server state before browser launch then a fresh Runtime consumes them once', async () => {
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
  await resumed.completeOidc('weknora://oidc?code=code-1&state=server-state');
  assert.equal(exchangeCalls.length, 1);
  assert.equal(resumed.snapshot().surface, 'upgrade-required');
});

test('concurrent native callbacks claim one persisted handoff and exchange it once', async () => {
  const pending = pendingStore();
  pending.value = { deploymentOrigin: DEPLOYMENT.origin, state: 'state-1', codeVerifier: 'verifier-1', redirectUri: 'weknora://oidc' };
  let exchanges = 0;
  const release = deferred<StoredCredential>();
  const oidcRemote = remote({
    oidcExchange: async () => { throw new Error('provider-code endpoint must not receive native handoff'); },
    oidcNativeExchange: async () => { exchanges += 1; return release.promise; },
  });
  const runtime = createMobileRuntime({ ...ports(fakeStore(), () => oidcRemote), pendingOidcStore: pending });

  const first = runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');
  const second = runtime.completeOidc('weknora://oidc?code=code-1&state=state-1');
  await Promise.resolve();
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
