import { describe, expect, it } from 'vitest';
import { createSecureStoreAeadCipher, type AeadBox } from './execution-storage.ts';

/**
 * MX-012 R1 P1 回归：AEAD 密钥初始化 single-flight。
 * 并发首次加密必须共享同一次密钥初始化——否则密文用 keyB 加密、SecureStore 落盘 keyA，
 * 产生永久不可解密文（审查者实测复现的竞态）。以异步 SecureStore 时序真实驱动。
 */

function xorAead(): AeadBox {
  return {
    randomBytes: (size) => {
      const bytes = new Uint8Array(size);
      for (let i = 0; i < size; i += 1) bytes[i] = (i * 31 + 7) & 0xff;
      return bytes;
    },
    encrypt: (message, _aad, nonce, key) => {
      const out = new Uint8Array(message.length);
      for (let i = 0; i < message.length; i += 1) out[i] = message[i]! ^ key[i % key.length]! ^ nonce[i % nonce.length]!;
      return out;
    },
    decrypt: (ciphertext, _aad, nonce, key) => {
      const out = new Uint8Array(ciphertext.length);
      for (let i = 0; i < ciphertext.length; i += 1) out[i] = ciphertext[i]! ^ key[i % key.length]! ^ nonce[i % nonce.length]!;
      return out;
    },
  };
}

function asyncStore() {
  const map = new Map<string, string>();
  return {
    map,
    getItemAsync: async (key: string) => {
      await Promise.resolve();
      await Promise.resolve();
      return map.get(key) ?? null;
    },
    setItemAsync: async (key: string, value: string) => {
      await Promise.resolve();
      map.set(key, value);
    },
  };
}

describe('MX-012 AEAD key single-flight', () => {
  it('concurrent first encrypts share one key initialization and all ciphertexts decrypt with the persisted key', async () => {
    const store = asyncStore();
    const cipher = createSecureStoreAeadCipher(store, xorAead());
    const payloadA = { text: '并发首个密文' };
    const payloadB = { text: '第二个并发密文' };
    // 并发首次加密（密钥尚未初始化）
    const [sealedA, sealedB] = await Promise.all([
      cipher.encrypt(payloadA, 'scope\u0000run\u0000a'),
      cipher.encrypt(payloadB, 'scope\u0000run\u0000b'),
    ]);
    // 落盘密钥必须只有一把，且两份密文都能用它解开
    expect(store.map.size).toBe(1);
    const persisted = [...store.map.values()][0]!;
    expect(persisted).toBeTruthy();
    const reopened = createSecureStoreAeadCipher(store, xorAead());
    const decryptedA = await reopened.decrypt(sealedA, 'scope\u0000run\u0000a');
    const decryptedB = await reopened.decrypt(sealedB, 'scope\u0000run\u0000b');
    expect(decryptedA).toEqual(payloadA);
    expect(decryptedB).toEqual(payloadB);
    console.log(`MX012-SINGLEFLIGHT-OBSERVATION ${JSON.stringify({ persistedKeys: store.map.size, bothDecrypt: true })}`);
  });

  it('key initialization failure clears in-flight state so a retry can succeed', async () => {
    let failures = 0;
    const store = {
      getItemAsync: async () => {
        if (failures === 0) {
          failures += 1;
          throw new Error('secure store read failed');
        }
        return null;
      },
      setItemAsync: async () => undefined,
    };
    const cipher = createSecureStoreAeadCipher(store, xorAead());
    await expect(cipher.encrypt({ a: 1 }, 'aad')).rejects.toThrow('secure store read failed');
    const sealed = await cipher.encrypt({ a: 1 }, 'aad');
    expect(sealed.keyVersion).toBe(1);
  });
});
