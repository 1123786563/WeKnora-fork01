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

/** 与实现的 draftKeySegment 一致（Buffer 版，避免 btoa 的 Node 环境差异）。 */
const segment = (id: string): string => Buffer.from(id, 'binary').toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');

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
    if (key.endsWith(`.d.${segment('draft-1')}`)) assert.match(value, /^[A-Za-z0-9+/]+={0,2}$/);
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
  const scopeKey = await scopeKeyOf(SCOPE_A);
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
  const scopeKey = await scopeKeyOf(SCOPE_A);
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'keep-me' });
  const oldKey = keyStore.entries().get(scopeKey)!;

  await scoped.rotate(lease(SCOPE_A));

  const after = await (await scoped.open(lease(SCOPE_A))).drafts.get('draft-1');
  assert.equal(after?.body, 'keep-me');
  const newKey = keyStore.entries().get(scopeKey)!;
  assert.notDeepEqual([...newKey], [...oldKey]);
  // 旧 key 无法解密轮换后的行（真实 GCM 认证失败）
  const rowB64 = storage.entries().get(`${scopeKey}.d.${segment('draft-1')}`)!;
  const rowBytes = new Uint8Array(Buffer.from(rowB64, 'base64'));
  await assert.rejects(createWebCryptoCipher().open(oldKey, rowBytes), /VAULT_DECRYPT/);
});

test('tampered ciphertext and wrong keys fail closed through the cipher seam', async () => {
  const { vault: scoped, storage } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'integrity' });

  const rowKey = `${await scopeKeyOf(SCOPE_A)}.d.${segment('draft-1')}`;
  const bytes = Buffer.from(storage.entries().get(rowKey)!, 'base64');
  bytes[bytes.length - 1] ^= 0xff;
  (storage.entries() as Map<string, string>).set(rowKey, Buffer.from(bytes).toString('base64'));

  const reader = await scoped.open(lease(SCOPE_A));
  await assert.rejects(reader.drafts.get('draft-1'), /VAULT_DECRYPT/);
});

test('inspectPolicy exposes the cacheable category and its retention window', () => {
  const { vault: scoped } = vault();
  assert.deepEqual(scoped.inspectPolicy(), { categories: [{ category: 'drafts', retentionDays: 30 }, { category: 'projections', retentionDays: 30 }] });
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
  const scopeKey = await scopeKeyOf(SCOPE_A);
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'healthy' });
  await store.drafts.put({ id: 'draft-2', body: 'doomed' });

  const row1Before = storage.entries().get(`${scopeKey}.d.${segment('draft-1')}`)!;
  const keyBefore = new Uint8Array(keyStore.entries().get(scopeKey)!);
  const row2 = `${scopeKey}.d.${segment('draft-2')}`;
  const bytes = Buffer.from(storage.entries().get(row2)!, 'base64');
  bytes[bytes.length - 1] ^= 0xff;
  (storage.entries() as Map<string, string>).set(row2, Buffer.from(bytes).toString('base64'));

  await assert.rejects(scoped.rotate(lease(SCOPE_A)), /VAULT_DECRYPT/);

  // 原子中止：未换 key、未重封任何行，损坏只停留在损坏的那一行
  assert.deepEqual([...keyStore.entries().get(scopeKey)!], [...keyBefore]);
  assert.equal(storage.entries().get(`${scopeKey}.d.${segment('draft-1')}`), row1Before);
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

test('reserved index row cannot be hijacked: put/get/remove validate ids symmetrically', async () => {
  const { vault: scoped } = vault();
  const store = await scoped.open(lease());
  // R1-F10：id=index 不得命中索引键；R1-F40：get/remove 与 put 校验对称
  await assert.rejects(store.drafts.put({ id: 'index', body: 'x' }), /VAULT_ID/);
  await assert.rejects(store.drafts.get('index'), /VAULT_ID/);
  await assert.rejects(store.drafts.remove('index'), /VAULT_ID/);
  await assert.rejects(store.drafts.put({ id: 'a/b', body: 'x' }), /VAULT_ID/); // 非法字符
  await assert.rejects(store.drafts.get(''), /VAULT_ID/); // 空串
  const longId = 'x'.repeat(65);
  await assert.rejects(store.drafts.put({ id: longId, body: 'x' }), /VAULT_ID/); // 超 64
  await store.drafts.put({ id: 'y'.repeat(64), body: 'ok' }); // 上限仍可用
  assert.equal((await store.drafts.get('y'.repeat(64)))?.body, 'ok');
});

test('scope keys are SecureStore-safe and collision-free across dotted inputs (R1-F39/F15)', async () => {
  // origin 含点号/冒号：v1 分段编码会拼出相同 scopeKey；v2 摘要不冲突且仅含 [0-9a-f.]
  const a = await scopeKeyOf({ deploymentOrigin: 'https://a.b.test', userId: 'u.1', tenantId: '7' });
  const b = await scopeKeyOf({ deploymentOrigin: 'https://a.test', userId: 'b.u.1', tenantId: '7' });
  assert.notEqual(a, b);
  assert.match(a, /^weknora\.vault\.v2\.[0-9a-f]+$/);
});

test('revoke of one scope never leaks rows into another scope key', async () => {
  const { vault: scoped } = vault();
  const a = await scoped.open(lease());
  await a.drafts.put({ id: 'd1', body: 'secret-a' });
  const otherScope = { deploymentOrigin: 'https://other.example.test', userId: 'user-1', tenantId: '7' };
  const b = await scoped.open(lease(otherScope));
  await b.drafts.put({ id: 'd1', body: 'secret-b' });
  const revoked = lease(); // 新 lease（scope A）
  await scoped.revoke(revoked, 'dispose');
  assert.equal(await b.drafts.get('d1') !== undefined && (await b.drafts.get('d1'))?.body === 'secret-b', true);
});

test('concurrent first open shares one key (single-flight, R1-F41)', async () => {
  const { keyStore, storage, vault: scoped } = vault();
  let reads = 0;
  const slowKeyStore = {
    ...keyStore,
    readWrappedKey: async (k: string) => { reads += 1; await new Promise((r) => setTimeout(r, 5)); return keyStore.readWrappedKey(k); },
    writeWrappedKey: async (k: string, v: Uint8Array) => { await new Promise((r) => setTimeout(r, 5)); return keyStore.writeWrappedKey(k, v); },
  };
  const slow = createScopedVault({ keyStore: slowKeyStore, storage, cipher: createWebCryptoCipher() });
  await Promise.all([slow.open(lease()), slow.open(lease())]);
  await (await slow.open(lease())).drafts.put({ id: 'a', body: 'persisted' });
  const reopened = await slow.open(lease());
  assert.equal((await reopened.drafts.get('a'))?.body, 'persisted'); // 未持久化的 K1 不会再出现
  assert.ok(reads >= 1);
});

test('a wrapped key of wrong length fails closed instead of being overwritten (R1-F13)', async () => {
  const { keyStore, storage } = vault();
  const key = await scopeKeyOf(SCOPE_A);
  await keyStore.writeWrappedKey(key, new Uint8Array(31)); // 持久化值损坏/长度异常
  const scoped = createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() });
  await assert.rejects(scoped.open(lease()), /VAULT_KEYSTORE/);
});

test('concurrent puts do not lose index entries (R1-F12)', async () => {
  const { vault: scoped } = vault();
  const store = await scoped.open(lease());
  await Promise.all(Array.from({ length: 20 }, (_, i) => store.drafts.put({ id: `d${i}`, body: `b${i}` })));
  const ids = (await store.drafts.list()).map((e) => e.id).sort();
  assert.deepEqual(ids, Array.from({ length: 20 }, (_, i) => `d${i}`).sort());
});

test('a corrupt index fails closed instead of being silently orphaned (R1-F12)', async () => {
  const { keyStore, storage, vault: scoped } = vault();
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'd1', body: 'x' });
  const scopeKey = await scopeKeyOf(SCOPE_A);
  await storage.write(`${scopeKey}.ix`, '{not-json');
  await assert.rejects(store.drafts.list(), /VAULT_INDEX/);
  await assert.rejects(store.drafts.put({ id: 'd2', body: 'y' }), /VAULT_INDEX/); // 不得覆写索引孤儿化全部行
});

test('rotate holds the scope lock against concurrent puts (R1-F11)', async () => {
  const { vault: scoped } = vault();
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'd1', body: 'v1' });
  const rotated = scoped.rotate(lease());
  const put = store.drafts.put({ id: 'd2', body: 'v2' });
  await Promise.all([rotated, put]);
  assert.equal((await store.drafts.get('d1'))?.body, 'v1');
  assert.equal((await store.drafts.get('d2'))?.body, 'v2');
  const reopened = await scoped.open(lease()); // 新会话强制重读 keyStore
  assert.equal((await reopened.drafts.list()).length, 2); // 重启后无 VAULT_DECRYPT
});

test('revoke tolerates a corrupt index and still erases key material', async () => {
  const { storage, vault: scoped } = vault();
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'd1', body: 'x' });
  const scopeKey = await scopeKeyOf(SCOPE_A);
  await storage.write(`${scopeKey}.ix`, '{corrupt');
  await scoped.revoke(lease(), 'dispose');
  assert.equal(await storage.read(`${scopeKey}.ix`), null);
  assert.equal(await storage.read(`${scopeKey}.d.${segment('d1')}`), null);
});

test('a row relocated under another id is rejected (R1-F42)', async () => {
  const { storage, vault: scoped } = vault();
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'a', body: 'A' });
  const scopeKey = await scopeKeyOf(SCOPE_A);
  const rowA = await storage.read(`${scopeKey}.d.${segment('a')}`);
  await storage.write(`${scopeKey}.d.${segment('b')}`, rowA!); // 把 a 的密文挪到 b 的键下
  await assert.rejects(store.drafts.get('b'), /VAULT_DECRYPT/);
});

test('list prunes rows older than the retention window (R1-F14)', async () => {
  const { keyStore, storage } = vault();
  const real = new Date();
  let clock = new Date(real.getTime() - 31 * 24 * 3600 * 1000); // 受控时钟：31 天前
  const scoped = createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher(), now: () => clock.toISOString() });
  const store = await scoped.open(lease());
  await store.drafts.put({ id: 'stale', body: 'old' }); // updatedAt = 31 天前
  clock = real; // 回到现在
  await store.drafts.put({ id: 'fresh', body: 'keep' }); // updatedAt = 现在
  const scopeKey = await scopeKeyOf(SCOPE_A);
  const listed = await store.drafts.list();
  assert.deepEqual(listed.map((e) => e.id), ['fresh']);
  assert.equal(await storage.read(`${scopeKey}.d.${segment('stale')}`), null); // 已被清理
});

// ── T10（#40）：projections 仓储（event projection 加密缓存类别）──

test('projections round-trip rows and stay isolated from drafts by namespace', async () => {
  const { vault: scoped } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await store.projections.put({ id: 'run.42', body: '{"taskId":"task-42"}' });
  assert.equal((await store.projections.get('run.42'))?.body, '{"taskId":"task-42"}');
  assert.deepEqual((await store.projections.list()).map((entry) => entry.id), ['run.42']);
  assert.equal(await store.drafts.get('run.42'), undefined, 'the same id in another namespace must not alias');
  await store.projections.remove('run.42');
  assert.equal(await store.projections.get('run.42'), undefined);
  assert.deepEqual((await store.projections.list()), []);
});

test('projections are scope-isolated exactly like drafts', async () => {
  const { vault: scoped } = vault();
  const storeA = await scoped.open(lease(SCOPE_A));
  await storeA.projections.put({ id: 'run.42', body: 'tenant-one-projection' });
  const storeOtherDeployment = await scoped.open(lease(SCOPE_B));
  const storeOtherTenant = await scoped.open(lease({ ...SCOPE_A, tenantId: '9' }));
  const storeOtherUser = await scoped.open(lease({ ...SCOPE_A, userId: 'user-2' }));
  assert.equal(await storeOtherDeployment.projections.get('run.42'), undefined);
  assert.equal(await storeOtherTenant.projections.get('run.42'), undefined);
  assert.equal(await storeOtherUser.projections.get('run.42'), undefined);
  assert.equal((await storeA.projections.get('run.42'))?.body, 'tenant-one-projection');
});

test('projection rows are ciphertext on disk and revoked with the scope', async () => {
  const { vault: scoped, keyStore, storage } = vault();
  const store = await scoped.open(lease(SCOPE_A));
  await store.drafts.put({ id: 'draft-1', body: 'keep' });
  await store.projections.put({ id: 'run.42', body: 'projection-canary' });
  for (const [key, value] of storage.entries()) {
    if (key.includes('.p.')) {
      assert.doesNotMatch(value, /projection-canary/);
      assert.match(value, /^[A-Za-z0-9+/]+={0,2}$/, 'the projection row must be base64 ciphertext');
    }
  }
  await scoped.revoke(lease(SCOPE_A), 'sign-out');
  const scopeKey = await scopeKeyOf(SCOPE_A);
  for (const rowKey of storage.entries().keys()) assert.equal(rowKey.startsWith(scopeKey), false, `row ${rowKey} must be erased`);
  assert.equal(keyStore.entries().size, 0, 'the wrapped key must be erased');
  const reopened = await scoped.open(lease(SCOPE_A));
  assert.equal(await reopened.projections.get('run.42'), undefined);
  assert.equal(await reopened.drafts.get('draft-1'), undefined);
});

test('rotate re-keys projection rows together with drafts', async () => {
  const { vault: scoped, keyStore } = vault();
  const scopeKey = await scopeKeyOf(SCOPE_A);
  const store = await scoped.open(lease(SCOPE_A));
  await store.projections.put({ id: 'run.42', body: 'projection-body' });
  const oldKey = keyStore.entries().get(scopeKey)!;
  await scoped.rotate(lease(SCOPE_A));
  const after = await (await scoped.open(lease(SCOPE_A))).projections.get('run.42');
  assert.equal(after?.body, 'projection-body');
  assert.notDeepEqual([...keyStore.entries().get(scopeKey)!], [...oldKey]);
});
