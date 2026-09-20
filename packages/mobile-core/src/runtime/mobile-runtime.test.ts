import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileRuntime } from './mobile-runtime.ts';
import type { CredentialStore, MobileRuntimePorts, RuntimeRemote, StoredCredential } from './ports.ts';
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
    ...overrides,
  };
}

function ports(store = fakeStore(), remoteFor = (_deployment: string) => remote()): MobileRuntimePorts {
  return { credentialStore: store, remoteFor, clientVersion: 3 };
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
