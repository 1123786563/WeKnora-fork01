import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileRuntime } from './mobile-runtime.ts';
import type { CredentialStore, MobileRuntimePorts, RuntimeRemote, StoredCredential } from './ports.ts';

const DEPLOYMENT = 'https://weknora.example.test';
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
    passwordLogin: async () => ({ accessToken: 'access-1', refreshToken: 'refresh-1' }),
    me: async () => ({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }),
    deploymentCapabilities: async () => FULL_CAPABILITIES,
    ...overrides,
  };
}

function ports(store = fakeStore(), remoteFor = (_deployment: string) => remote()): MobileRuntimePorts {
  return { credentialStore: store, remoteFor, clientVersion: 3 };
}

test('boot restores credentials before identity and capabilities', async () => {
  const order: string[] = [];
  const store = fakeStore({ [DEPLOYMENT]: { accessToken: 'stored-access', refreshToken: 'stored-refresh' } });
  const runtime = createMobileRuntime(ports(store, () => remote({
    me: async () => { order.push('me'); return { user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }; },
    deploymentCapabilities: async () => { order.push('capabilities'); return FULL_CAPABILITIES; },
  })));

  await runtime.boot(DEPLOYMENT);

  assert.deepEqual(store.calls, [`read:${DEPLOYMENT}`]);
  assert.deepEqual(order, ['me', 'capabilities']);
  assert.equal(runtime.snapshot().surface, 'full');
});

test('sign in saves deployment-scoped credentials before loading identity and capabilities', async () => {
  const order: string[] = [];
  const store = fakeStore();
  const runtime = createMobileRuntime(ports(store, () => remote({
    passwordLogin: async () => { order.push('login'); return { accessToken: 'access-1', refreshToken: 'refresh-1' }; },
    me: async () => { order.push('me'); return { user: { id: 'user-1' }, tenant: { id: 'tenant-1' } }; },
    deploymentCapabilities: async () => { order.push('capabilities'); return FULL_CAPABILITIES; },
  })));

  await runtime.signIn(DEPLOYMENT, { email: 'member@example.test', password: 'password' });

  assert.deepEqual(order, ['login', 'me', 'capabilities']);
  assert.deepEqual(store.calls, [`write:${DEPLOYMENT}`]);
  assert.equal(runtime.snapshot().surface, 'full');
  assert.ok(runtime.snapshot().scopeLease);
});

test('only a full compatibility mode grants the authorized surface', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({
    deploymentCapabilities: async () => ({ protocol_minimum: 4, protocol_maximum: 4 }),
  })));

  await runtime.signIn(DEPLOYMENT, { email: 'member@example.test', password: 'password' });

  assert.equal(runtime.snapshot().surface, 'upgrade-required');
  assert.equal(runtime.snapshot().scopeLease, undefined);
});

test('malformed and unknown capabilities remain on the upgrade-required surface', async () => {
  for (const capability of [{ protocol_minimum: '2', protocol_maximum: 3 }, { version: 'future' }]) {
    const runtime = createMobileRuntime(ports(fakeStore(), () => remote({ deploymentCapabilities: async () => capability })));
    await runtime.signIn(DEPLOYMENT, { email: 'member@example.test', password: 'password' });
    assert.equal(runtime.snapshot().surface, 'upgrade-required');
    assert.equal(runtime.snapshot().scopeLease, undefined);
  }
});

test('a missing active tenant stays safe', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({
    me: async () => ({ user: { id: 'user-1' }, tenant: null }),
  })));

  await runtime.signIn(DEPLOYMENT, { email: 'member@example.test', password: 'password' });

  assert.equal(runtime.snapshot().surface, 'upgrade-required');
  assert.equal(runtime.snapshot().scopeLease, undefined);
});

test('a deployment change revokes its prior lease and never sends prior credentials to the new transport', async () => {
  const otherDeployment = 'https://other.example.test';
  const seen: Array<{ deployment: string; accessToken: string }> = [];
  const runtime = createMobileRuntime(ports(fakeStore(), (deployment) => remote({
    passwordLogin: async () => ({ accessToken: deployment === DEPLOYMENT ? 'first-access' : 'second-access', refreshToken: 'refresh-1' }),
    me: async (accessToken) => {
      seen.push({ deployment, accessToken });
      return { user: { id: deployment }, tenant: { id: deployment } };
    },
  })));

  await runtime.signIn(DEPLOYMENT, { email: 'member@example.test', password: 'password' });
  const oldLease = runtime.snapshot().scopeLease!;
  await runtime.signIn(otherDeployment, { email: 'member@example.test', password: 'password' });

  assert.equal(oldLease.isActive(), false);
  assert.deepEqual(seen, [
    { deployment: DEPLOYMENT, accessToken: 'first-access' },
    { deployment: otherDeployment, accessToken: 'second-access' },
  ]);
});

test('late responses after sign out are ignored', async () => {
  const identity = deferred<{ user: { id: string }; tenant: { id: string } }>();
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote({ me: async () => identity.promise })));

  const signIn = runtime.signIn(DEPLOYMENT, { email: 'member@example.test', password: 'password' });
  await Promise.resolve();
  await runtime.signOut();
  identity.resolve({ user: { id: 'user-1' }, tenant: { id: 'tenant-1' } });
  await signIn;

  assert.equal(runtime.snapshot().surface, 'signed-out');
  assert.equal(runtime.snapshot().scopeLease, undefined);
});
