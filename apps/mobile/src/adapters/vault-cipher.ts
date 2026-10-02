import { gcm } from '@noble/ciphers/aes.js';
import type { CipherPort } from '@weknora/mobile-core';

const IV_LENGTH = 12;
const KEY_LENGTH = 32;

/** T39 #69 D5: Hermes（Release/真机 JS 引擎）没有 crypto.subtle，WebCrypto
 * cipher 抛 VAULT_CIPHER_UNAVAILABLE 后整个 vault fail-soft 成 undefined——
 * 草稿/任务投影全部丢失。@noble/ciphers 的纯 JS AES-GCM 不依赖 subtle，
 * 线格式与 web-crypto-cipher 完全一致（IV‖密文），既有密文互相可解。 */
export function createHermesVaultCipher(): CipherPort {
  return {
    async seal(key, plaintext) {
      if (key.length !== KEY_LENGTH) throw new Error('VAULT_KEY_LENGTH');
      const iv = new Uint8Array(IV_LENGTH);
      globalThis.crypto.getRandomValues(iv);
      const sealed = gcm(key, iv).encrypt(plaintext);
      const out = new Uint8Array(IV_LENGTH + sealed.length);
      out.set(iv);
      out.set(sealed, IV_LENGTH);
      return out;
    },
    async open(key, ciphertext) {
      if (key.length !== KEY_LENGTH) throw new Error('VAULT_KEY_LENGTH');
      if (ciphertext.length <= IV_LENGTH) throw new Error('VAULT_DECRYPT');
      try {
        return gcm(key, ciphertext.slice(0, IV_LENGTH)).decrypt(ciphertext.slice(IV_LENGTH));
      } catch {
        throw new Error('VAULT_DECRYPT');
      }
    },
  };
}
