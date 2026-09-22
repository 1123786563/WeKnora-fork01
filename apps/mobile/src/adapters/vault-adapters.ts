import type { KeyStorePort, VaultStoragePort } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

/** OS-backed wrapped-key storage; the vault scope key doubles as the SecureStore key. */
export function createSecureVaultKeyStore(store: SecureStorePort): KeyStorePort {
  return {
    async readWrappedKey(scopeKey) { const raw = await store.getItemAsync(scopeKey); return raw === null ? undefined : base64ToBytes(raw); },
    async writeWrappedKey(scopeKey, key) { await store.setItemAsync(scopeKey, bytesToBase64(key)); },
    async deleteWrappedKey(scopeKey) { await store.deleteItemAsync(scopeKey); },
  };
}

/** OS-backed encrypted-row storage; values are opaque base64 ciphertext from the vault. */
export function createSecureVaultStorage(store: SecureStorePort): VaultStoragePort {
  return {
    async read(key) { return store.getItemAsync(key); },
    async write(key, value) { await store.setItemAsync(key, value); },
    async delete(key) { await store.deleteItemAsync(key); },
  };
}
