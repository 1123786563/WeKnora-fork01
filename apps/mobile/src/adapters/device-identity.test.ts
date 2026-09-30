import test from 'node:test';
import assert from 'node:assert/strict';
import type { SecureStorePort } from './secure-store.ts';
import { createSecureDeviceIdentity } from './device-identity.ts';

test('concurrent first deviceId calls resolve to one persisted identity', async () => {
  const writes: string[] = [];
  const store: SecureStorePort = {
    getItemAsync: async () => null,
    setItemAsync: async (_key, value) => { writes.push(value); },
    deleteItemAsync: async () => {},
  };
  const identity = createSecureDeviceIdentity(store);
  const [a, b, c] = await Promise.all([identity.deviceId(), identity.deviceId(), identity.deviceId()]);
  assert.equal(a, b);
  assert.equal(b, c);
  assert.equal(writes.length, 1, 'B3-F49：并发首次调用只落一次盘，不产生幽灵设备记录');
});

test('a persisted identity is reused without new writes', async () => {
  const writes: string[] = [];
  let stored: string | null = 'device-persisted';
  const store: SecureStorePort = {
    getItemAsync: async () => stored,
    setItemAsync: async (_key, value) => { writes.push(value); stored = value; },
    deleteItemAsync: async () => {},
  };
  const identity = createSecureDeviceIdentity(store);
  assert.equal(await identity.deviceId(), 'device-persisted');
  assert.equal(writes.length, 0);
});
