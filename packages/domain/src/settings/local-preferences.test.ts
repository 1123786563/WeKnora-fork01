import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  readLocalPreferences,
  writeLocalPreferences,
  isValidTheme,
  isValidFontSize,
  migratePreferencesIntoUser,
  resetMigrationLatch,
  readUserIdFromStorage,
} from './local-preferences.ts';

/** Minimal Storage double with removal support (port of Vue preferenceStorage). */
function mem(seed?: Record<string, string>) {
  const m = new Map<string, string>(Object.entries(seed ?? {}));
  return {
    getItem: (k: string) => (m.has(k) ? (m.get(k) as string) : null),
    setItem: (k: string, v: string) => void m.set(k, v),
    removeItem: (k: string) => void m.delete(k),
    dump: () => Object.fromEntries(m.entries()),
  };
}

function asUser(s: ReturnType<typeof mem>, id: string | null) {
  if (id === null) s.removeItem('weknora_user');
  else s.setItem('weknora_user', JSON.stringify({ id }));
  resetMigrationLatch();
}

test('defaults when storage is empty', () => {
  const p = readLocalPreferences(mem());
  // Vue useTheme.ts:14-15 — missing stored theme resolves to 'light' (浅色),
  // the authoritative default; 'system' is only an explicit user choice.
  assert.equal(p.theme, 'light');
  assert.equal(p.fontSize, 'normal');
});

test('round-trips theme, locale and font size under the active user namespace', () => {
  const s = mem();
  asUser(s, 'u1');
  writeLocalPreferences(s, { theme: 'dark', locale: 'ja-JP', fontSize: 'large' });
  const p = readLocalPreferences(s);
  assert.equal(p.theme, 'dark');
  assert.equal(p.locale, 'ja-JP');
  assert.equal(p.fontSize, 'large');
  // Vue preferenceStorage.ts — keys are WeKnora_${userId}_${suffix}; locale stays flat.
  assert.equal(s.getItem('WeKnora_u1_theme'), 'dark');
  assert.equal(s.getItem('WeKnora_u1_font_size'), 'large');
  assert.equal(s.getItem('locale'), 'ja-JP');
});

test('different userIds read isolated preferences (G2: no cross-account bleed)', () => {
  const s = mem();
  asUser(s, 'u1');
  writeLocalPreferences(s, { theme: 'dark', fontSize: 'large' });
  asUser(s, 'u2');
  const p = readLocalPreferences(s);
  assert.equal(p.theme, 'light');
  assert.equal(p.fontSize, 'normal');
});

test('userId resolution: weknora_user JSON id, anon fallback on missing/corrupt', () => {
  const s = mem();
  assert.equal(readUserIdFromStorage(s), 'anon');
  s.setItem('weknora_user', JSON.stringify({ id: 42 }));
  assert.equal(readUserIdFromStorage(s), '42');
  s.setItem('weknora_user', '{broken');
  assert.equal(readUserIdFromStorage(s), 'anon');
  s.setItem('weknora_user', JSON.stringify({ nope: true }));
  assert.equal(readUserIdFromStorage(s), 'anon');
});

test('legacy un-namespaced Vue key (WeKnora_theme) migrates into user namespace', () => {
  const s = mem({ WeKnora_theme: 'dark', WeKnora_font_size: 'large' });
  asUser(s, 'u1');
  migratePreferencesIntoUser(s);
  const p = readLocalPreferences(s);
  assert.equal(p.theme, 'dark');
  assert.equal(p.fontSize, 'large');
  // Source keys are always removed so later users cannot inherit them.
  assert.equal(s.getItem('WeKnora_theme'), null);
  assert.equal(s.getItem('WeKnora_font_size'), null);
});

test('React flat keys (weknora-theme / weknora-font-size) migrate into user namespace', () => {
  const s = mem({ 'weknora-theme': 'dark', 'weknora-font-size': 'large' });
  asUser(s, 'u1');
  migratePreferencesIntoUser(s);
  const p = readLocalPreferences(s);
  assert.equal(p.theme, 'dark');
  assert.equal(p.fontSize, 'large');
  assert.equal(s.getItem('weknora-theme'), null);
  assert.equal(s.getItem('weknora-font-size'), null);
});

test('anon namespace is adopted (and cleared) for a logged-in user', () => {
  const s = mem({ WeKnora_anon_theme: 'dark' });
  asUser(s, 'u1');
  migratePreferencesIntoUser(s);
  assert.equal(readLocalPreferences(s).theme, 'dark');
  assert.equal(s.getItem('WeKnora_anon_theme'), null);
});

test('anon values win over legacy flat keys during migration', () => {
  const s = mem({ WeKnora_anon_theme: 'dark', 'weknora-theme': 'light', WeKnora_theme: 'system' });
  asUser(s, 'u1');
  migratePreferencesIntoUser(s);
  assert.equal(readLocalPreferences(s).theme, 'dark');
});

test('existing user key is never overwritten, sources still cleaned up', () => {
  const s = mem({
    WeKnora_u1_theme: 'light',
    WeKnora_anon_theme: 'dark',
    WeKnora_theme: 'system',
  });
  asUser(s, 'u1');
  migratePreferencesIntoUser(s);
  assert.equal(readLocalPreferences(s).theme, 'light');
  assert.equal(s.getItem('WeKnora_anon_theme'), null);
  assert.equal(s.getItem('WeKnora_theme'), null);
});

test('anon (no logged-in user) migration is a no-op and keeps anon prefs readable', () => {
  const s = mem({ WeKnora_anon_theme: 'dark' });
  asUser(s, null);
  migratePreferencesIntoUser(s);
  // anon keeps its own namespace; nothing is migrated or deleted.
  assert.equal(s.getItem('WeKnora_anon_theme'), 'dark');
  assert.equal(readLocalPreferences(s).theme, 'dark');
});

test('migration is idempotent per user per session (latch), reset re-runs it', () => {
  const calls: string[] = [];
  const s = mem();
  const orig = s.removeItem;
  s.removeItem = (k: string) => { calls.push(k); orig(k); };
  asUser(s, 'u1');
  s.setItem('WeKnora_theme', 'dark');
  migratePreferencesIntoUser(s);
  assert.equal(calls.filter((k) => k === 'WeKnora_theme').length, 1);
  // Second call same user: latch short-circuits, no extra writes.
  migratePreferencesIntoUser(s);
  assert.equal(calls.filter((k) => k === 'WeKnora_theme').length, 1);
  // User switch: latch reset re-enables migration (nothing left to adopt).
  asUser(s, 'u2');
  migratePreferencesIntoUser(s);
  assert.equal(readLocalPreferences(s).theme, 'light');
});

test('locale stays flat across users (Vue parity: locale key is not namespaced)', () => {
  const s = mem();
  asUser(s, 'u1');
  writeLocalPreferences(s, { locale: 'en-US' });
  asUser(s, 'u2');
  assert.equal(readLocalPreferences(s).locale, 'en-US');
});

test('invalid values fall back to defaults', () => {
  const s = mem();
  asUser(s, 'u1');
  s.setItem('WeKnora_u1_theme', 'bogus');
  s.setItem('WeKnora_u1_font_size', 'huge');
  const p = readLocalPreferences(s);
  // Vue useTheme.ts:14-15 — invalid stored theme also resolves to 'light'.
  assert.equal(p.theme, 'light');
  assert.equal(p.fontSize, 'normal');
});

test('isValidTheme and isValidFontSize gate correctly', () => {
  assert.ok(isValidTheme('light'));
  assert.ok(!isValidTheme('bogus'));
  assert.ok(isValidFontSize('large'));
  assert.ok(!isValidFontSize('huge'));
});
