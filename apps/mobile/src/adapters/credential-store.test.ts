import test from 'node:test';
import assert from 'node:assert/strict';
import type { SecureStorePort } from './secure-store.ts';
import { createSecureCredentialStore } from './credential-store.ts';

test('带端口 origin 的凭据键保持在 SecureStore 合法字符集内', async () => {
  const keys: string[] = [];
  const values = new Map<string, string>();
  const store: SecureStorePort = {
    getItemAsync: async (key) => { keys.push(key); return values.get(key) ?? null; },
    setItemAsync: async (key, value) => { keys.push(key); values.set(key, value); },
    deleteItemAsync: async (key) => { keys.push(key); values.delete(key); },
  };
  const credentials = createSecureCredentialStore(store);
  const origin = 'https://192-168-3-33.nip.io:8443';
  await credentials.write(origin, { token: 't', refreshToken: 'r' });
  const read = await credentials.read(origin);
  await credentials.clear(origin);
  for (const key of keys) {
    assert.match(key, /^[A-Za-z0-9._-]+$/, `SecureStore 键含非法字符: ${key}`);
  }
  assert.ok(read && read.token === 't', '同一 origin 读回写入的凭据');
  assert.equal(keys[0], keys[2], 'write 与 clear 派生同一键');
});
