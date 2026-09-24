import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScopedVault } from '../vault/scoped-vault.ts';
import { createWebCryptoCipher } from '../vault/web-crypto-cipher.ts';
import { createInMemoryVaultKeyStore, createInMemoryVaultStorage } from '../vault/in-memory-adapters.ts';
import { createVaultTaskProjectionStore, PROJECTION_BODY_BUDGET_BYTES } from './vault-projection-store.ts';
import type { PersistedTaskProjection, TaskBackendEvent } from './task-detail.ts';

const SCOPE = { deploymentOrigin: 'https://a.example.test', userId: 'user-1', tenantId: '7' };

const event$ = (seq: number): TaskBackendEvent => ({
  runId: 'run-42', seq, type: 'text.delta', occurredAt: '2026-09-24T00:00:00Z', payload: { note: 'x'.repeat(60) },
});

const projection$ = (events: TaskBackendEvent[], cursor: number): PersistedTaskProjection => ({
  taskId: 'task-42', runId: 'run-42', cursor, events, savedAt: '2026-09-24T00:00:00Z',
  snapshot: {
    taskId: 'task-42', runId: 'run-42', title: '离线快照标题', attention: 'none',
    execution: { runStatus: 'succeeded', executionStatus: 'succeeded', settlementStatus: 'settled', revision: 3, seq: cursor },
    watermark: cursor, incomplete: false,
  },
});

function leasedVault() {
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  const vault = createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() });
  let revocable = new RuntimeScopeLease(SCOPE);
  const lease = () => revocable.asScopeLease();
  return {
    storage, vault, lease,
    store: createVaultTaskProjectionStore({ vault, lease }),
    revokeLease: () => { revocable.revoke(); },
  };
}

test('saves and loads a full projection round-trip through the encrypted vault', async () => {
  const { store } = leasedVault();
  const projection = projection$([event$(1), event$(2)], 2);
  await store.save(projection);
  const loaded = await store.load('run-42');
  assert.deepEqual(loaded, projection);
});

test('projection rows are ciphertext and never contain the plaintext snapshot', async () => {
  const { store, storage } = leasedVault();
  await store.save(projection$([event$(1)], 1));
  const rows = [...storage.entries().values()].filter((value) => !value.startsWith('[')); // 排除索引行（明文 id 属预期）
  assert.ok(rows.length > 0);
  for (const value of rows) {
    assert.doesNotMatch(value, /离线快照标题/);
    assert.doesNotMatch(value, /task-42/);
  }
});

test('oversized projections shed the oldest events within budget and keep the snapshot intact', async () => {
  const { store } = leasedVault();
  const events = Array.from({ length: 100 }, (_unused, index) => event$(index + 1));
  await store.save(projection$(events, 100));
  const loaded = await store.load('run-42');
  assert.ok(loaded);
  assert.ok(loaded!.events.length < 100, 'oversized projections must be trimmed, not rejected or silently dropped');
  const seqs = loaded!.events.map((event) => event.seq);
  assert.ok(seqs.every((seq, index) => index === 0 || seq === seqs[index - 1]! + 1), 'the retained events are a contiguous recent window');
  assert.equal(seqs[seqs.length - 1], 100, 'the newest event is always kept');
  assert.equal(loaded!.snapshot?.title, '离线快照标题', 'the snapshot survives trimming');
  assert.equal(loaded!.cursor, 100, 'the committed cursor survives trimming');
  // 预算自证：序列化体（含同样裁剪后的 events）在预算内
  const serialized = JSON.stringify({ ...loaded });
  assert.ok(new TextEncoder().encode(serialized).length <= PROJECTION_BODY_BUDGET_BYTES + 200, `serialized body ${serialized.length}B stays near budget`);
});

test('a revoked or missing lease fails closed on save and load', async () => {
  const { store, revokeLease } = leasedVault();
  await store.save(projection$([event$(1)], 1));
  revokeLease();
  await assert.rejects(store.load('run-42'), /VAULT_LEASE/);
  await assert.rejects(store.save(projection$([event$(2)], 2)), /VAULT_LEASE/);
});

test('a runtime-style vault revoke erases the projection rows for the scope (AC2 revocation face)', async () => {
  // Runtime 在撤权/退出/切租户时串行派发 vault.revoke（#32 mobile-runtime.ts:116-135）；
  // 本测试显式执行同一序列，验证投影行随之不可读（先覆写 key 再删行的既有语义）。
  const { vault, lease, store } = leasedVault();
  await store.save(projection$([event$(1)], 1));
  await vault.revoke(lease(), 'sign-out');
  assert.equal(await store.load('run-42'), undefined, 'a revoked scope leaves no readable projection');
  await store.save(projection$([event$(1)], 1)); // 同 scope 重新落盘 = 全新 key 全新行
  assert.equal((await store.load('run-42'))?.events.length, 1);
});

test('a tampered projection row fails closed through the cipher', async () => {
  const { store, storage } = leasedVault();
  await store.save(projection$([event$(1)], 1));
  const rowKey = [...storage.entries().keys()].find((key) => key.includes('.p.'))!;
  const bytes = Buffer.from(storage.entries().get(rowKey)!, 'base64');
  bytes[bytes.length - 1] ^= 0xff;
  (storage.entries() as Map<string, string>).set(rowKey, Buffer.from(bytes).toString('base64'));
  await assert.rejects(store.load('run-42'), /VAULT_DECRYPT/);
});

test('malformed run ids never touch the vault', async () => {
  const { store } = leasedVault();
  assert.equal(await store.load('../etc/passwd'), undefined);
  assert.equal(await store.load(''), undefined);
  await assert.rejects(store.save({ ...projection$([], 0), runId: '../etc/passwd' }), /VAULT_RUN_ID/);
});
