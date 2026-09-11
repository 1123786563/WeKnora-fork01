import assert from 'node:assert/strict';
import test from 'node:test';
import { createOriginStorage } from './origin-storage';
test('selected origin survives restart and can be cleared', async () => {
  const values = new Map<string, string>();
  const store = { get: async (k: string) => values.get(k) ?? null, set: async (k: string, v: string) => { values.set(k, v); }, remove: async (k: string) => { values.delete(k); } };
  const first = createOriginStorage(store);
  await first.write('https://one.example');
  assert.equal(await createOriginStorage(store).read(), 'https://one.example');
  await first.clear();
  assert.equal(await first.read(), null);
});
