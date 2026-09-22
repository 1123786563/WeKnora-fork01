import type { CipherPort } from './ports.ts';

const IV_LENGTH = 12;
const KEY_LENGTH = 32;

/** AES-GCM via Web Crypto. IV is prepended to the sealed bytes; any wrong key or tamper fails the GCM tag. */
export function createWebCryptoCipher(): CipherPort {
  const subtle = globalThis.crypto?.subtle;
  if (!subtle) throw new Error('VAULT_CIPHER_UNAVAILABLE');
  /** Subtle 只接受 ArrayBuffer 视图；把调用方传入的 Uint8Array 拷贝到纯 ArrayBuffer 视图（TS BufferSource 兼容）。 */
  const buffer = (bytes: Uint8Array): Uint8Array<ArrayBuffer> => {
    const copy = new Uint8Array(bytes.byteLength);
    copy.set(bytes);
    return copy;
  };
  const importKey = (key: Uint8Array, usage: KeyUsage[]) => subtle.importKey('raw', buffer(key), 'AES-GCM', false, usage);
  return {
    async seal(key, plaintext) {
      if (key.length !== KEY_LENGTH) throw new Error('VAULT_KEY_LENGTH');
      const iv = globalThis.crypto.getRandomValues(new Uint8Array(IV_LENGTH));
      const sealed = new Uint8Array(await subtle.encrypt({ name: 'AES-GCM', iv }, await importKey(key, ['encrypt']), buffer(plaintext)));
      const out = new Uint8Array(IV_LENGTH + sealed.length);
      out.set(iv);
      out.set(sealed, IV_LENGTH);
      return out;
    },
    async open(key, ciphertext) {
      if (key.length !== KEY_LENGTH) throw new Error('VAULT_KEY_LENGTH');
      if (ciphertext.length <= IV_LENGTH) throw new Error('VAULT_DECRYPT');
      try {
        const iv = buffer(ciphertext.slice(0, IV_LENGTH));
        const body = buffer(ciphertext.slice(IV_LENGTH));
        return new Uint8Array(await subtle.decrypt({ name: 'AES-GCM', iv }, await importKey(key, ['decrypt']), body));
      } catch {
        throw new Error('VAULT_DECRYPT');
      }
    },
  };
}
