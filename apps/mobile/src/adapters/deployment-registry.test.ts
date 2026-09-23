import test from 'node:test';
import assert from 'node:assert/strict';
import { createSecureDeploymentRegistry } from './deployment-registry.ts';
import type { SecureStorePort } from './secure-store.ts';

function secureStore(): SecureStorePort & { values: Map<string, string> } {
  const values = new Map<string, string>();
  return {
    values,
    async getItemAsync(key) { return values.get(key) ?? null; },
    async setItemAsync(key, value) { values.set(key, value); },
    async deleteItemAsync(key) { values.delete(key); },
  };
}

test('secure deployment registry upserts most-recent-first without duplicates', async () => {
  const store = secureStore();
  const registry = createSecureDeploymentRegistry(store);

  await registry.upsert({ origin: 'https://weknora.example.test', label: 'WeKnora' });
  await registry.upsert({ origin: 'https://other.example.test', label: 'Other' });
  await registry.upsert({ origin: 'https://weknora.example.test', label: 'WeKnora Cloud' });

  assert.deepEqual(await registry.list(), [
    { origin: 'https://weknora.example.test', label: 'WeKnora Cloud' },
    { origin: 'https://other.example.test', label: 'Other' },
  ]);
});

test('secure deployment registry removes one origin and keeps the rest', async () => {
  const store = secureStore();
  const registry = createSecureDeploymentRegistry(store);
  await registry.upsert({ origin: 'https://weknora.example.test', label: 'WeKnora' });
  await registry.upsert({ origin: 'https://other.example.test', label: 'Other' });

  await registry.remove('https://weknora.example.test');

  assert.deepEqual(await registry.list(), [{ origin: 'https://other.example.test', label: 'Other' }]);
  await registry.remove('https://missing.example.test');
  assert.deepEqual(await registry.list(), [{ origin: 'https://other.example.test', label: 'Other' }]);
});

test('a malformed registry payload reads as an empty list and is rebuilt on the next upsert', async () => {
  const store = secureStore();
  await store.setItemAsync('weknora.deployment-registry.v1', '{"origin":"https://not-an-array.test"}');
  const registry = createSecureDeploymentRegistry(store);

  assert.deepEqual(await registry.list(), []);

  await registry.upsert({ origin: 'https://weknora.example.test', label: 'WeKnora' });

  assert.deepEqual(await registry.list(), [{ origin: 'https://weknora.example.test', label: 'WeKnora' }]);
});

test('registry rows never store credentials and fall back to the origin as the label', async () => {
  const store = secureStore();
  const registry = createSecureDeploymentRegistry(store);
  await registry.upsert({ origin: 'https://weknora.example.test', label: '' });

  const raw = JSON.parse(store.values.get('weknora.deployment-registry.v1')!) as Array<Record<string, unknown>>;
  assert.deepEqual(raw, [{ origin: 'https://weknora.example.test', label: 'https://weknora.example.test' }]);
  assert.deepEqual(await registry.list(), [{ origin: 'https://weknora.example.test', label: 'https://weknora.example.test' }]);
});

// 共享夹具：内存 SecureStore 记录最后一次写入
function recordingStore(initial: string | null = null): { store: SecureStorePort; writes: Array<string | null> } {
  const writes: Array<string | null> = [initial];
  return {
    writes,
    store: {
      getItemAsync: async () => writes[0] ?? null,
      setItemAsync: async (_key: string, value: string) => { writes[0] = value; },
      deleteItemAsync: async () => { writes[0] = null; },
    },
  };
}

test('list skips malformed entries and keeps the valid ones', async () => {
  const raw = JSON.stringify([
    { origin: 'https://a.example.test', label: 'A' },
    'not-an-object',                                   // 畸形：整体曾返回 undefined → 清空全部
    { origin: '', label: 'empty-origin' },             // 畸形 origin
    { origin: 'https://b.example.test' },              // 合法：label 兜底为 origin
  ]);
  const { store } = recordingStore(raw);
  const registry = createSecureDeploymentRegistry(store);
  const listed = await registry.list();
  assert.deepEqual(listed, [
    { origin: 'https://a.example.test', label: 'A' },
    { origin: 'https://b.example.test', label: 'https://b.example.test' },
  ]);
});

test('upsert over a corrupt store recovers instead of wiping siblings', async () => {
  // 预置：一条合法 + 一条畸形；upsert 新实例后，畸形条目被剔除、合法条目保留
  const { store, writes } = recordingStore(JSON.stringify([{ origin: 'https://a.example.test', label: 'A' }, 42]));
  const registry = createSecureDeploymentRegistry(store);
  await registry.upsert({ origin: 'https://new.example.test', label: 'New' });
  const persisted = JSON.parse(writes[0]!);
  assert.deepEqual(persisted.map((r: { origin: string }) => r.origin), ['https://new.example.test', 'https://a.example.test']);
});

test('upsert caps the registry at MAX_REGISTRY_ENTRIES entries', async () => {
  const { store, writes } = recordingStore();
  const registry = createSecureDeploymentRegistry(store);
  for (let index = 0; index < 12; index += 1) await registry.upsert({ origin: `https://host${index}.example.test`, label: `h${index}` });
  const persisted = JSON.parse(writes[0]!);
  assert.equal(persisted.length, 8, '注册表必须有界：超出上限裁掉最旧条目，保证 SecureStore 单值不超 2KB 级预算');
  assert.equal(persisted[0].origin, 'https://host11.example.test'); // 前移语义：最新在最前
});

test('a non-array payload degrades to an empty registry without throwing', async () => {
  const { store } = recordingStore('"just-a-string"');
  const registry = createSecureDeploymentRegistry(store);
  assert.deepEqual(await registry.list(), []);
});
