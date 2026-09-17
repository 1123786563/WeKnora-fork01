// R441 A4 full-chain regression: login → change theme → logout → second
// account logs in — each account must resolve its OWN theme (per-user
// WeKnora_${userId}_* namespaces keyed off weknora_user).
//
// Covered chain (mirrors the runtime sequence):
//   persistLogin (auth/api.ts)          → writes weknora_user  [login]
//   migratePreferencesIntoUser (domain) → adopts anon/legacy   [post-login]
//   writeLocalPreferences (domain)      → per-user writes      [settings]
//   persistBrowserCredential anonymous  → clears weknora_user  [logout/401]
import assert from 'node:assert/strict';
import test from 'node:test';

import { persistLogin, type ParsedLogin } from './api.ts';
import { persistBrowserCredential } from '../platform/credentials.ts';
import {
  clearWeknoraUser,
  migratePreferencesIntoUser,
  readLocalPreferences,
  readUserIdFromStorage,
  resetMigrationLatch,
  writeLocalPreferences,
} from '@weknora/domain/settings/local-preferences';

function storage() {
  const m = new Map<string, string>();
  return {
    getItem: (k: string) => (m.has(k) ? m.get(k)! : null),
    setItem: (k: string, v: string) => void m.set(k, v),
    removeItem: (k: string) => void m.delete(k),
    dump: () => Object.fromEntries(m.entries()),
  };
}

function bearerSession(userId: string): ParsedLogin {
  return {
    credential: { kind: 'bearer', accessToken: `token-${userId}`, refreshToken: `refresh-${userId}` },
    tenantId: '1',
    user: { id: userId, email: `${userId}@test.dev`, nickname: userId },
  };
}

function login(s: ReturnType<typeof storage>, userId: string) {
  persistLogin(bearerSession(userId), s);
  resetMigrationLatch();
  migratePreferencesIntoUser(s);
}

function logout(s: ReturnType<typeof storage>) {
  // main.tsx logout() → browserCredentialAdapter.clear() → anonymous write;
  // scope-runtime.logout() also clears the identity (defence in depth).
  persistBrowserCredential(s, { kind: 'anonymous' });
  clearWeknoraUser(s);
  resetMigrationLatch();
}

test('persistLogin writes weknora_user (full login response user, Vue setUser parity)', () => {
  const s = storage();
  persistLogin(bearerSession('u1'), s);
  assert.deepEqual(JSON.parse(s.getItem('weknora_user')!), { id: 'u1', email: 'u1@test.dev', nickname: 'u1' });
  assert.equal(readUserIdFromStorage(s), 'u1');
  assert.equal(s.getItem('weknora_token'), 'token-u1');
});

test('persistLogin without a user payload leaves an existing identity untouched', () => {
  const s = storage();
  s.setItem('weknora_user', JSON.stringify({ id: 'keep-me' }));
  persistLogin({ credential: { kind: 'bearer', accessToken: 't', refreshToken: 'r' }, tenantId: null }, s);
  assert.deepEqual(JSON.parse(s.getItem('weknora_user')!), { id: 'keep-me' });
});

test('full chain: u1 and u2 keep independent themes across login/logout cycles', () => {
  const s = storage();

  // u1 logs in, adopts whatever anon left, and picks dark.
  login(s, 'u1');
  writeLocalPreferences(s, { theme: 'dark' });
  assert.equal(readLocalPreferences(s).theme, 'dark');

  // u1 logs out: identity drops, preferences stay under WeKnora_u1_*.
  logout(s);
  assert.equal(readUserIdFromStorage(s), 'anon');
  assert.equal(s.getItem('WeKnora_u1_theme'), 'dark');

  // u2 logs in: default theme — u1's dark must NOT bleed across accounts.
  login(s, 'u2');
  assert.equal(readLocalPreferences(s).theme, 'light');
  writeLocalPreferences(s, { theme: 'system' });

  // u2 logs out; u1 returns and sees their own dark back.
  logout(s);
  login(s, 'u1');
  assert.equal(readLocalPreferences(s).theme, 'dark');

  // u1 logs out; u2 returns and sees system, still isolated from u1.
  logout(s);
  login(s, 'u2');
  assert.equal(readLocalPreferences(s).theme, 'system');
});

test('anon pre-login theme is adopted by the FIRST login and not by the next', () => {
  const s = storage();
  // Anonymous visitor tunes the theme before ever logging in.
  writeLocalPreferences(s, { theme: 'dark' });
  assert.equal(s.getItem('WeKnora_anon_theme'), 'dark');

  login(s, 'u1');
  assert.equal(readLocalPreferences(s).theme, 'dark');
  assert.equal(s.getItem('WeKnora_anon_theme'), null, 'anon namespace is consumed so the next user cannot inherit it');

  logout(s);
  login(s, 'u2');
  assert.equal(readLocalPreferences(s).theme, 'light');
});

// R462 A3: the two adapter contract tests migrated here from auth-state.test.ts
// (R461 A2) were removed together with createAuthApi -- production login goes
// through @weknora/api-client's createAuthApi, whose endpoint contracts are
// pinned by packages/api-client/src/auth/endpoints.test.ts.
