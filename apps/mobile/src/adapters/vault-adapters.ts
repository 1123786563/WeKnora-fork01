import type { KeyStorePort, VaultStoragePort } from '@weknora/mobile-core';
import { base64ToBytes, bytesToBase64 } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

/** OS-backed wrapped-key storage; the vault scope key doubles as the SecureStore key. */
export function createSecureVaultKeyStore(store: SecureStorePort): KeyStorePort {
  return {
    // 损坏值（非 base64）不得被当作「无 key」触发新生成——那会静默锁死既有密文（R1-F18）。
    async readWrappedKey(scopeKey) {
      const raw = await store.getItemAsync(scopeKey);
      if (raw === null) return undefined;
      try {
        return base64ToBytes(raw);
      } catch {
        throw new Error('VAULT_KEYSTORE');
      }
    },
    async writeWrappedKey(scopeKey, key) { await store.setItemAsync(scopeKey, bytesToBase64(key)); },
    async deleteWrappedKey(scopeKey) { await store.deleteItemAsync(scopeKey); },
  };
}

/** Android expo-secure-store 每值约 2048 字节软限制：超限静默丢写，显式失败优于静默（R1-F17）。 */
const SECURE_STORE_MAX_VALUE_BYTES = 2000;

/** OS-backed encrypted-row storage; values are opaque base64 ciphertext from the vault. */
export function createSecureVaultStorage(store: SecureStorePort): VaultStoragePort {
  return {
    async read(key) { return store.getItemAsync(key); },
    async write(key, value) {
      if (value.length > SECURE_STORE_MAX_VALUE_BYTES) throw new Error('VAULT_ROW_TOO_LARGE');
      await store.setItemAsync(key, value);
    },
    async delete(key) { await store.deleteItemAsync(key); },
  };
}
