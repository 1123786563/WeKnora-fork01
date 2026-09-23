/** Symmetric authenticated-encryption seam. `open` must fail on wrong key or tampered ciphertext. */
export interface CipherPort {
  seal(key: Uint8Array, plaintext: Uint8Array): Promise<Uint8Array>;
  open(key: Uint8Array, ciphertext: Uint8Array): Promise<Uint8Array>;
}

/** Persists wrapped per-scope data keys (OS keychain/SecureStore in the app, memory in tests). */
export interface KeyStorePort {
  readWrappedKey(scopeKey: string): Promise<Uint8Array | undefined>;
  writeWrappedKey(scopeKey: string, key: Uint8Array): Promise<void>;
  deleteWrappedKey(scopeKey: string): Promise<void>;
}

/** Persists opaque encrypted rows; keys carry no plaintext semantics (SQLite/SecureStore/memory adapters). */
export interface VaultStoragePort {
  read(key: string): Promise<string | null>;
  write(key: string, value: string): Promise<void>;
  delete(key: string): Promise<void>;
}

export interface ScopedVaultPorts {
  keyStore: KeyStorePort;
  storage: VaultStoragePort;
  cipher: CipherPort;
  /** 32 bytes of entropy per fresh data key; omit only where Web Crypto is available. */
  randomBytes?: (size: number) => Uint8Array;
  /** 受控时钟（测试注入）；缺省 new Date().toISOString()。行的 updatedAt 与保留清理共用。 */
  now?: () => string;
}
