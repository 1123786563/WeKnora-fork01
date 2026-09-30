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
