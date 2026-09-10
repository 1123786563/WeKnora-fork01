import assert from 'node:assert/strict';
import test from 'node:test';

import { createCredentials } from './credentials';

function store() {
  const values = new Map<string, string>();
  return {
    values,
    get: async (key: string) => values.get(key) ?? null,
    set: async (key: string, value: string) => { values.set(key, value); },
    remove: async (key: string) => { values.delete(key); },
  };
}

test('credentials survive a new adapter instance and clear without leaking plaintext', async () => {
  const backing = store();
  const first = createCredentials(backing, 'weknora:origin-a');
  await first.write({ kind: 'bearer', accessToken: 'a', refreshToken: 'r' });
  assert.deepEqual(await createCredentials(backing, 'weknora:origin-a').read(), {
    kind: 'bearer', accessToken: 'a', refreshToken: 'r',
  });
  assert.match(backing.values.get('weknora:origin-a') ?? '', /accessToken/);
  await first.clear();
  assert.deepEqual(await first.read(), { kind: 'anonymous' });
});

test('credential keys isolate product origins', async () => {
  const backing = store();
  const a = createCredentials(backing, 'weknora:origin-a');
  const b = createCredentials(backing, 'weknora:origin-b');
  await a.write({ kind: 'bearer', accessToken: 'a' });
  assert.deepEqual(await b.read(), { kind: 'anonymous' });
  await b.write({ kind: 'bearer', accessToken: 'b' });
  assert.equal((await a.read()).kind, 'bearer');
  assert.equal((await b.read()).kind, 'bearer');
});

test('invalid stored values are treated as anonymous', async () => {
  const backing = store();
  backing.values.set('key', '{"kind":"bearer","accessToken":""}');
  assert.deepEqual(await createCredentials(backing, 'key').read(), { kind: 'anonymous' });
});
