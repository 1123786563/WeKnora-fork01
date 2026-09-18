import assert from 'node:assert/strict';
import test from 'node:test';

import { createBrowserCredentialAdapter, persistBrowserCredential } from './credentials.ts';

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

// R441 A4: the anonymous write IS the logout path — it must also drop the
// weknora_user identity so the next login starts in a fresh anon namespace
// (Vue stores/auth.ts logout removes weknora_user).
test('anonymous credential write clears weknora_user; bearer writes keep it', async () => {
  const browserStorage = storage({ weknora_token: 't', weknora_user: JSON.stringify({ id: 'u1', email: 'a@b.c' }) });

  await persistBrowserCredential(browserStorage, { kind: 'bearer', accessToken: 't2' });
  assert.deepEqual(JSON.parse(browserStorage.getItem('weknora_user') ?? 'null'), { id: 'u1', email: 'a@b.c' });

  await persistBrowserCredential(browserStorage, { kind: 'anonymous' });
  assert.equal(browserStorage.getItem('weknora_user'), null);
});

test('embed credential write keeps weknora_user (shared browser, embed session)', async () => {
  const browserStorage = storage({ weknora_user: JSON.stringify({ id: 'u1' }) });
  await persistBrowserCredential(browserStorage, { kind: 'embed', token: 'embed x' });
  assert.notEqual(browserStorage.getItem('weknora_user'), null);
});
