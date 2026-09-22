import test from 'node:test';
import assert from 'node:assert/strict';
import type { ScopeLease } from '../runtime/types.ts';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { createScopedVault, scopeKeyOf } from './scoped-vault.ts';
import { createWebCryptoCipher } from './web-crypto-cipher.ts';
import { createInMemoryVaultKeyStore, createInMemoryVaultStorage } from './in-memory-adapters.ts';

const SCOPE_A = { deploymentOrigin: 'https://a.example.test', userId: 'user-1', tenantId: '7' };
const SCOPE_B = { deploymentOrigin: 'https://b.example.test', userId: 'user-1', tenantId: '7' };

function lease(scope = SCOPE_A): ScopeLease & { revoke(): void } {
  return new RuntimeScopeLease(scope) as unknown as ScopeLease & { revoke(): void };
}

function vault() {
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  return {
    keyStore,
    storage,
    vault: createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() }),
  };
}

test('drafts are isolated by deployment, user, and tenant scope', async () => {
  const { vault: scoped } = vault();
  const storeA = await scoped.open(lease(SCOPE_A));
  await storeA.drafts.put({ id: 'draft-1', body: 'alpha' });

  const storeOtherDeployment = await scoped.open(lease(SCOPE_B));
  const storeOtherTenant = await scoped.open(lease({ ...SCOPE_A, tenantId: '9' }));
  const storeOtherUser = await scoped.open(lease({ ...SCOPE_A, userId: 'user-2' }));

  assert.equal(await storeOtherDeployment.drafts.get('draft-1'), undefined);
  assert.equal(await storeOtherTenant.drafts.get('draft-1'), undefined);
  assert.equal(await storeOtherUser.drafts.get('draft-1'), undefined);
  assert.equal((await storeA.drafts.get('draft-1'))?.body, 'alpha');
  assert.deepEqual((await storeA.drafts.list()).map((entry) => entry.id), ['draft-1']);
});

test('stored rows never contain plaintext bodies', async () => {
  const { vault: scoped, storage } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'plaintext-canary' });

  for (const [key, value] of storage.entries()) {
    assert.doesNotMatch(value, /plaintext-canary/);
    // 索引行只含不透明 id（明文 JSON 属预期）；草稿行必须是 base64 密文
    if (key.endsWith('.drafts.draft-1')) assert.match(value, /^[A-Za-z0-9+/]+={0,2}$/);
  }
});

test('a vault without a valid lease fails closed on open and on every store operation', async () => {
  const { vault: scoped } = vault();
  await assert.rejects(scoped.open({} as ScopeLease), /VAULT_LEASE/);

  const handle = lease(SCOPE_A);
  const revoked = new RuntimeScopeLease(SCOPE_A);
  revoked.revoke();
  await assert.rejects(scoped.open(revoked.asScopeLease()), /VAULT_LEASE/);

  const store = await scoped.open(handle);
  await store.drafts.put({ id: 'draft-1', body: 'secret' });
  handle.revoke();
  await assert.rejects(store.drafts.put({ id: 'draft-2', body: 'x' }), /VAULT_LEASE/);
  await assert.rejects(store.drafts.get('draft-1'), /VAULT_LEASE/);
  await assert.rejects(store.drafts.list(), /VAULT_LEASE/);
  await assert.rejects(store.drafts.remove('draft-1'), /VAULT_LEASE/);
});

test('revoke rotates the wrapped key to unusable randomness and erases rows for the scope', async () => {
  const { vault: scoped, keyStore, storage } = vault();
  const scopeKey = scopeKeyOf(SCOPE_A);
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'tenant-secret' });
  const keyBefore = keyStore.entries().get(scopeKey);
  assert.ok(keyBefore);

  await scoped.revoke(lease(SCOPE_A), 'tenant-switch');

  // 行与索引均被擦除；wrapped key 即使残留也是新随机值，读不出旧密文
  for (const rowKey of storage.entries().keys()) assert.equal(rowKey.startsWith(scopeKey), false, `row ${rowKey} must be erased`);
  const keyAfter = keyStore.entries().get(scopeKey);
  if (keyAfter) assert.notDeepEqual([...keyAfter], [...keyBefore!]);
  const reopened = await scoped.open(lease(SCOPE_A));
  assert.equal(await reopened.drafts.get('draft-1'), undefined);
});

test('rotate re-keys the scope while preserving readable drafts', async () => {
  const { vault: scoped, keyStore, storage } = vault();
  const scopeKey = scopeKeyOf(SCOPE_A);
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'keep-me' });
  const oldKey = keyStore.entries().get(scopeKey)!;

  await scoped.rotate(lease(SCOPE_A));

  const after = await (await scoped.open(lease(SCOPE_A))).drafts.get('draft-1');
  assert.equal(after?.body, 'keep-me');
  const newKey = keyStore.entries().get(scopeKey)!;
  assert.notDeepEqual([...newKey], [...oldKey]);
  // 旧 key 无法解密轮换后的行（真实 GCM 认证失败）
  const rowB64 = storage.entries().get(`${scopeKey}.drafts.draft-1`)!;
  const rowBytes = new Uint8Array(Buffer.from(rowB64, 'base64'));
  await assert.rejects(createWebCryptoCipher().open(oldKey, rowBytes), /VAULT_DECRYPT/);
});

test('tampered ciphertext and wrong keys fail closed through the cipher seam', async () => {
  const { vault: scoped, storage } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'integrity' });

  const rowKey = `${scopeKeyOf(SCOPE_A)}.drafts.draft-1`;
  const bytes = Buffer.from(storage.entries().get(rowKey)!, 'base64');
  bytes[bytes.length - 1] ^= 0xff;
  (storage.entries() as Map<string, string>).set(rowKey, Buffer.from(bytes).toString('base64'));

  const reader = await scoped.open(lease(SCOPE_A));
  await assert.rejects(reader.drafts.get('draft-1'), /VAULT_DECRYPT/);
});

test('inspectPolicy exposes the cacheable category and its retention window', () => {
  const { vault: scoped } = vault();
  assert.deepEqual(scoped.inspectPolicy(), { categories: [{ category: 'drafts', retentionDays: 30 }] });
});

test('a failing encryption adapter rejects the write and leaves no row behind', async () => {
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  const failingCipher = {
    seal: async () => { throw new Error('cipher adapter down'); },
    open: async () => { throw new Error('cipher adapter down'); },
  };
  const scoped = createScopedVault({ keyStore, storage, cipher: failingCipher });
  const store = await scoped.open(lease(SCOPE_A));

  await assert.rejects(store.drafts.put({ id: 'draft-1', body: 'x' }), /cipher adapter down/);

  assert.equal([...storage.entries().keys()].filter((key) => key.endsWith('.draft-1')).length, 0, 'no row may be written when sealing fails');
});

test('rotate aborts atomically on a corrupt row, leaving healthy rows readable under the old key', async () => {
  const { vault: scoped, keyStore, storage } = vault();
  const scopeKey = scopeKeyOf(SCOPE_A);
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'healthy' });
  await store.drafts.put({ id: 'draft-2', body: 'doomed' });

  const row1Before = storage.entries().get(`${scopeKey}.drafts.draft-1`)!;
  const keyBefore = new Uint8Array(keyStore.entries().get(scopeKey)!);
  const row2 = `${scopeKey}.drafts.draft-2`;
  const bytes = Buffer.from(storage.entries().get(row2)!, 'base64');
  bytes[bytes.length - 1] ^= 0xff;
  (storage.entries() as Map<string, string>).set(row2, Buffer.from(bytes).toString('base64'));

  await assert.rejects(scoped.rotate(lease(SCOPE_A)), /VAULT_DECRYPT/);

  // 原子中止：未换 key、未重封任何行，损坏只停留在损坏的那一行
  assert.deepEqual([...keyStore.entries().get(scopeKey)!], [...keyBefore]);
  assert.equal(storage.entries().get(`${scopeKey}.drafts.draft-1`), row1Before);
  const reader = await scoped.open(lease(SCOPE_A));
  assert.equal((await reader.drafts.get('draft-1'))?.body, 'healthy');
});

test('draft ids outside the safe alphabet are rejected before any storage write', async () => {
  const { vault: scoped, storage } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await assert.rejects(store.drafts.put({ id: 'bad id with spaces', body: 'x' }), /VAULT_ID/);
  await assert.rejects(store.drafts.put({ id: 'x'.repeat(65), body: 'x' }), /VAULT_ID/);
  assert.equal(storage.entries().size, 0, 'neither row nor index may be written for an invalid id');
});
