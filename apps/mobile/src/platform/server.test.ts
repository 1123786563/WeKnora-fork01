import assert from 'node:assert/strict';
import test from 'node:test';
import { createServerAddressAdapter } from './server.ts';

function store(initial: string | null = null) {
  let value = initial;
  return {
    async getItemAsync() { return value; },
    async setItemAsync(_key: string, next: string) { value = next; },
    async deleteItemAsync() { value = null; },
  };
}

test('reads and normalizes the persisted mobile server address', async () => {
  const adapter = createServerAddressAdapter(store('https://weknora.example.test/'));
  assert.equal(await adapter.read(), 'https://weknora.example.test');
});

test('writes only HTTP(S) server addresses and clears an empty value', async () => {
  const backing = store();
  const adapter = createServerAddressAdapter(backing);
  await adapter.write('http://10.0.2.2:8080/');
  assert.equal(await adapter.read(), 'http://10.0.2.2:8080');
  await adapter.write('   ');
  assert.equal(await adapter.read(), '');
  await assert.rejects(() => adapter.write('file:///tmp/weknora'), /HTTP\(S\)/);
});
