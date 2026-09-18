import test from 'node:test';
import assert from 'node:assert/strict';
import { createScopedKVSubmissionStore, type KVLike } from './submission-store.ts';
import type { SubmissionEntry, SubmissionScope } from '@weknora/domain/mobile';

function memoryKV(): KVLike & { dump(): Map<string, string> } {
  const map = new Map<string, string>();
  return {
    getString: (key) => map.get(key),
    set: (key, value) => { map.set(key, value); },
    delete: (key) => { map.delete(key); },
    getAllKeys: () => [...map.keys()],
    dump: () => map,
  };
}

const scopeA: SubmissionScope = { origin: 'https://a.example', tenantID: 't1', userID: 'u1' };
const scopeB: SubmissionScope = { origin: 'https://a.example', tenantID: 't2', userID: 'u1' };

function entry(scope: SubmissionScope, requestId: string): SubmissionEntry {
  return { request_id: requestId, input_digest: 'fnv1a:abc', scope, phase: 'awaiting_ack', updated_at: '2026-09-18T00:00:00Z' };
}

test('kv submission store round-trips within its scope and isolates others', () => {
  const kv = memoryKV();
  const storeA = createScopedKVSubmissionStore(kv, scopeA);
  storeA.save(entry(scopeA, 'req-1'));
  assert.equal(storeA.load('req-1')?.input_digest, 'fnv1a:abc');
  assert.equal(storeA.listScope(scopeA).length, 1);

  // 不同空间：新 store 读不到 A 的键，也不允许写入
  const storeB = createScopedKVSubmissionStore(kv, scopeB);
  assert.equal(storeB.load('req-1'), undefined);
  assert.deepEqual(storeB.listScope(scopeA), []);
  assert.throws(() => storeB.save(entry(scopeA, 'req-2')), /scope-bound/);
});

test('corrupted entries fail closed instead of surfacing partial state', () => {
  const kv = memoryKV();
  const store = createScopedKVSubmissionStore(kv, scopeA);
  store.save(entry(scopeA, 'req-1'));
  // 模拟底层损坏：写入非法 JSON
  const key = [...kv.dump().keys()][0];
  kv.set(key, '{not-json');
  assert.equal(store.load('req-1'), undefined);
  assert.deepEqual(store.listScope(scopeA), []);
});
