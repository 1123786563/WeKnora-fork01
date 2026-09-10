import assert from 'node:assert/strict';
import test from 'node:test';

import { createBrowserCredentialAdapter } from './credentials.ts';

function storage(initial: Record<string, string> = {}): Storage {
  const values = { ...initial };
  return {
    getItem(key) { return values[key] ?? null; },
    setItem(key, value) { values[key] = value; },
    removeItem(key) { delete values[key]; },
    clear() { for (const key of Object.keys(values)) delete values[key]; },
    key(index) { return Object.keys(values)[index] ?? null; },
    get length() { return Object.keys(values).length; },
  };
}

test('persists rotated bearer credentials and clears both token generations', async () => {
  const browserStorage = storage({ weknora_token: 'old-access', weknora_refresh_token: 'old-refresh' });
  const credentials = createBrowserCredentialAdapter(browserStorage);

  assert.deepEqual(await credentials.read(), { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' });
  await credentials.write({ kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
  assert.deepEqual(await credentials.read(), { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' });
  await credentials.clear();
  assert.deepEqual(await credentials.read(), { kind: 'anonymous' });
});

test('never retains a bearer refresh token for an Embed profile', async () => {
  const browserStorage = storage({ weknora_refresh_token: 'must-clear' });
  const credentials = createBrowserCredentialAdapter(browserStorage);

  await credentials.write({ kind: 'embed', token: 'Embed visitor-token' });
  assert.deepEqual(await credentials.read(), { kind: 'embed', token: 'Embed visitor-token' });
  assert.equal(browserStorage.getItem('weknora_refresh_token'), null);
});
