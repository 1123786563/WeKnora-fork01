import type { KeyStorePort, VaultStoragePort } from './ports.ts';

/** In-memory KeyStorePort for Module scenario tests (mirrors createInMemoryCredentialStore). */
export function createInMemoryVaultKeyStore(): KeyStorePort & { entries(): ReadonlyMap<string, Uint8Array> } {
  const values = new Map<string, Uint8Array>();
  return {
    entries: () => values,
    async readWrappedKey(scopeKey) { const value = values.get(scopeKey); return value && new Uint8Array(value); },
    async writeWrappedKey(scopeKey, key) { values.set(scopeKey, new Uint8Array(key)); },
    async deleteWrappedKey(scopeKey) { values.delete(scopeKey); },
  };
}

/** In-memory VaultStoragePort for Module scenario tests. */
export function createInMemoryVaultStorage(): VaultStoragePort & { entries(): ReadonlyMap<string, string> } {
  const values = new Map<string, string>();
  return {
    entries: () => values,
    async read(key) { return values.get(key) ?? null; },
    async write(key, value) { values.set(key, value); },
    async delete(key) { values.delete(key); },
  };
}
