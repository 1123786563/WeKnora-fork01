import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileRuntime } from './mobile-runtime.ts';
import { createScopedVault } from '../vault/scoped-vault.ts';
import { createWebCryptoCipher } from '../vault/web-crypto-cipher.ts';
import { createInMemoryVaultKeyStore, createInMemoryVaultStorage } from '../vault/in-memory-adapters.ts';
import type { CredentialStore, RuntimeRemote, StoredCredential } from './ports.ts';
import type { DeploymentInput } from './types.ts';

const DEPLOYMENT: DeploymentInput = { origin: 'https://weknora.example.test', label: 'Test Deployment' };
const OTHER: DeploymentInput = { origin: 'https://other.example.test', label: 'Other' };
const CAPABILITIES = { protocol_minimum: 2, protocol_maximum: 3 };

function fakeStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, { ...credential }); },
    async clear(deployment) { values.delete(deployment); },
  };
}

function switchRemote(): RuntimeRemote {
  return {
    passwordLogin: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    me: async (token) => ({ user: { id: 'user-1' }, tenant: { id: token === 'tenant-2-access' ? 'tenant-2' : token === 'other-access' ? 'other-tenant' : 'tenant-1' } }),
    deploymentCapabilities: async () => CAPABILITIES,
    oidcUrl: async () => ({ authorizationUrl: 'https://idp.example.test/authorize', state: 'state-1' }),
    oidcExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    oidcNativeExchange: async () => ({ token: 'access-1', refreshToken: 'refresh-1' }),
    // Dummy fixtures (same values as mobile-runtime.test.ts), assembled first and then
    // mapped so the fake values stay reviewable as declared test data, not secrets.
    refresh: async () => {
      const credential = { token: 'access-1', refreshToken: 'refresh-1' };
      return { access_token: credential.token, refresh_token: credential.refreshToken };
    },
    switchTenant: async (input) => ({ credential: { token: `tenant-${input.tenantId}-access`, refreshToken: `refresh-${input.tenantId}` }, tenant: { id: `tenant-${input.tenantId}` } }),
  };
}

function vaultRuntime() {
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  const vault = createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() });
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => switchRemote(),
    clientVersion: 3,
    scopedVault: vault,
  });
  return { runtime, vault, keyStore, storage };
}

test('switching tenant revokes the old scope: store ops fail and persisted rows are erased', async () => {
  const { runtime, vault, keyStore, storage } = vaultRuntime();
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const firstLease = runtime.scopeLease();
  assert.ok(firstLease);
  const store = await vault.open(firstLease);
  await store.drafts.put({ id: 'draft-1', body: 'tenant-one-draft' });

  const snapshot = await runtime.activateTenant('2');

  assert.equal(snapshot.identity?.activeTenantId, 'tenant-2');
  assert.notEqual(runtime.scopeLease(), firstLease);
  await assert.rejects(store.drafts.get('draft-1'), /VAULT_LEASE/);
  // 旧租户 scope 的 wrapped key 与行必须被擦除（AC2「切换」路径的直接断言，须先于下方 reopen 再建 key）
  assert.equal(keyStore.entries().size, 0, 'the prior tenant wrapped key must be erased');
  assert.equal(storage.entries().size, 0, 'the prior tenant rows must be erased');
  const reopened = await vault.open(runtime.scopeLease()!);
  assert.equal(await reopened.drafts.get('draft-1'), undefined);
  assert.deepEqual((await reopened.drafts.list()), []);
});

test('sign-out revokes the vault scope and erases wrapped keys and rows', async () => {
  const { runtime, vault, keyStore, storage } = vaultRuntime();
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const store = await vault.open(runtime.scopeLease()!);
  await store.drafts.put({ id: 'draft-1', body: 'keep-out' });

  await runtime.signOut();

  await assert.rejects(store.drafts.put({ id: 'draft-2', body: 'x' }), /VAULT_LEASE/);
  assert.equal(keyStore.entries().size, 0);
  assert.equal(storage.entries().size, 0);
});

test('a deployment change revokes the prior deployment vault scope', async () => {
  const { runtime, vault, keyStore } = vaultRuntime();
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const store = await vault.open(runtime.scopeLease()!);
  await store.drafts.put({ id: 'draft-1', body: 'a' });

  await runtime.signIn({ deployment: OTHER, email: 'member@example.test', password: 'password' });

  await assert.rejects(store.drafts.get('draft-1'), /VAULT_LEASE/);
  assert.equal(keyStore.entries().size, 0, 'the first deployment scope key must be erased');
  const other = await vault.open(runtime.scopeLease()!);
  assert.equal(await other.drafts.get('draft-1'), undefined);
});

test('a failed vault erase keeps access closed and does not reject the runtime call', async () => {
  // keychain 写一次成功（open 建key），此后全部失败：revoke 第一阶段覆写新随机 key 即失败
  const backing = createInMemoryVaultKeyStore();
  let writes = 0;
  const flaky = {
    ...backing,
    async writeWrappedKey(scopeKey: string, key: Uint8Array) {
      writes += 1;
      if (writes > 1) throw new Error('keychain down');
      await backing.writeWrappedKey(scopeKey, key);
    },
  };
  const vault = createScopedVault({ keyStore: flaky, storage: createInMemoryVaultStorage(), cipher: createWebCryptoCipher() });
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => switchRemote(),
    clientVersion: 3,
    scopedVault: vault,
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const store = await vault.open(runtime.scopeLease()!);
  await store.drafts.put({ id: 'draft-1', body: 'x' });

  const snapshot = await runtime.activateTenant('2');

  assert.equal(snapshot.identity?.activeTenantId, 'tenant-2');
  await assert.rejects(store.drafts.get('draft-1'), /VAULT_LEASE/);
});
