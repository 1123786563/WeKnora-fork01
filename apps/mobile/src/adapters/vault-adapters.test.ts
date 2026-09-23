import test from 'node:test';
import assert from 'node:assert/strict';
import { createSecureVaultKeyStore, createSecureVaultStorage } from './vault-adapters.ts';
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

test('secure vault key store round-trips wrapped keys as base64 under the scope key', async () => {
  const store = secureStore();
  const keys = createSecureVaultKeyStore(store);
  const key = new Uint8Array([1, 2, 3, 250, 255]);

  await keys.writeWrappedKey('scope-1', key);

  assert.match(store.values.get('scope-1')!, /^[A-Za-z0-9+/]+={0,2}$/);
  assert.deepEqual(await keys.readWrappedKey('scope-1'), key);
  await keys.deleteWrappedKey('scope-1');
  assert.equal(await keys.readWrappedKey('scope-1'), undefined);
  assert.equal(store.values.has('scope-1'), false);
});

test('secure vault storage delegates opaque rows without transformation', async () => {
  const store = secureStore();
  const storage = createSecureVaultStorage(store);
  await storage.write('row-1', 'opaque-ciphertext');
  assert.equal(await storage.read('row-1'), 'opaque-ciphertext');
  assert.equal(await storage.read('missing'), null);
  await storage.delete('row-1');
  assert.equal(await storage.read('row-1'), null);
});

test('a corrupt wrapped-key value fails closed with VAULT_KEYSTORE (R1-F18)', async () => {
  const store = secureStore();
  await store.setItemAsync('scope-key', '!!!not-base64!!!');
  const keyStore = createSecureVaultKeyStore(store);
  await assert.rejects(keyStore.readWrappedKey('scope-key'), /VAULT_KEYSTORE/);
});

test('oversized rows fail loudly instead of vanishing (R1-F17)', async () => {
  const store = secureStore();
  const storage = createSecureVaultStorage(store);
  await assert.rejects(storage.write('k', 'x'.repeat(2001)), /VAULT_ROW_TOO_LARGE/);
});
