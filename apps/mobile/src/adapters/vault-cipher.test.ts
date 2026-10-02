import { strict as assert } from 'node:assert';
import test from 'node:test';

import { createHermesVaultCipher } from './vault-cipher.ts';
import { createWebCryptoCipher } from '@weknora/mobile-core';

const key = new Uint8Array(32).fill(0xab);
const plaintext = new TextEncoder().encode('keep-draft-积分周报');

test('hermes vault cipher roundtrips and detects tampering', async () => {
  const cipher = createHermesVaultCipher();
  const sealed = await cipher.seal(key, plaintext);
  assert.deepEqual(new Uint8Array(await cipher.open(key, sealed)), plaintext);

  sealed[sealed.length - 1] ^= 0xff;
  await assert.rejects(cipher.open(key, sealed), /VAULT_DECRYPT/);
});

test('hermes vault cipher never reuses an IV and rejects bad key lengths', async () => {
  const cipher = createHermesVaultCipher();
  const ivs = new Set<string>();
  for (let i = 0; i < 8; i++) {
    const sealed = await cipher.seal(key, plaintext);
    ivs.add(Buffer.from(sealed.slice(0, 12)).toString('hex'));
  }
  assert.equal(ivs.size, 8, 'each seal must draw a fresh IV');
  await assert.rejects(cipher.seal(new Uint8Array(31), plaintext), /VAULT_KEY_LENGTH/);
  await assert.rejects(cipher.open(new Uint8Array(16), sealed0()), /VAULT_KEY_LENGTH/);

  function sealed0(): Uint8Array {
    return new Uint8Array(13);
  }
});

// T39 #69 D5: 同一线格式保证 WebCrypto 时期（远程调试态）封存的行仍可解。
test('hermes cipher reads rows sealed by the webcrypto cipher and vice versa', async () => {
  if (!globalThis.crypto?.subtle) return; // Node 环境恒有；此处只做跨实现互操作锚定
  const web = createWebCryptoCipher();
  const native = createHermesVaultCipher();
  const sealedByWeb = await web.seal(key, plaintext);
  assert.deepEqual(new Uint8Array(await native.open(key, sealedByWeb)), plaintext);
  const sealedByNative = await native.seal(key, plaintext);
  assert.deepEqual(new Uint8Array(await web.open(key, sealedByNative)), plaintext);
});
