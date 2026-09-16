// Port of Vue GeneralSettings.vue local preference handling: theme, language,
// sans/mono font, font size — all client-side (localStorage + CSS variables).
//
// Per-user namespace (G2 parity with Vue frontend/src/composables/preferenceStorage.ts):
// theme and font preferences live under `WeKnora_${userId}_${suffix}` where
// userId comes from the `weknora_user` storage entry (or "anon" before
// login). Read paths are intentionally narrow — no cross-namespace fallbacks
// — so one user's preferences cannot bleed into another user's session.
// The `locale` key stays flat (Vue i18n also reads the un-namespaced
// `locale` entry).
export type ThemeMode = 'light' | 'dark' | 'system';
export type FontSize = 'small' | 'normal' | 'large';

const USER_ID_KEY = 'weknora_user';
const LOCALE_KEY = 'locale';

/** Suffixes migrated into a user namespace at login time (Vue parity). */
const PREFERENCE_SUFFIXES = ['theme', 'font_sans', 'font_mono', 'font_size'] as const;

export interface LocalPreferences {
  theme: ThemeMode;
  locale: string;
  fontSize: FontSize;
}

type Storage = {
  getItem(k: string): string | null;
  setItem(k: string, v: string): void;
  removeItem?(k: string): void;
};

function safeGet(storage: Storage, key: string): string | null {
  try { return storage.getItem(key); } catch { return null; }
}

function safeSet(storage: Storage, key: string, value: string): void {
  try { storage.setItem(key, value); } catch { /* quota / private mode: best-effort */ }
}

function safeRemove(storage: Storage, key: string): void {
  try { storage.removeItem?.(key); } catch { /* best-effort */ }
}

/** Resolve the active user id from storage; "anon" before login (Vue parity). */
export function readUserIdFromStorage(storage: Storage): string {
  const raw = safeGet(storage, USER_ID_KEY);
  if (!raw) return 'anon';
  try {
    const parsed: unknown = JSON.parse(raw);
    const id = (parsed as { id?: unknown } | null)?.id;
    return id ? String(id) : 'anon';
  } catch {
    return 'anon';
  }
}

/** `WeKnora_${userId}_${suffix}` key template (Vue preferenceStorage.userKey). */
export function userKey(storage: Storage, suffix: string): string {
  return `WeKnora_${readUserIdFromStorage(storage)}_${suffix}`;
}

export function readLocalPreferences(storage: Storage): LocalPreferences {
  // Vue useTheme.ts:14-15 — a missing or invalid stored theme resolves to
  // 'light'; 'system' is only an explicit user choice.
  const theme = (safeGet(storage, userKey(storage, 'theme')) ?? 'light') as ThemeMode;
  const locale = safeGet(storage, LOCALE_KEY) ?? 'zh-CN';
  const fontSize = (safeGet(storage, userKey(storage, 'font_size')) ?? 'normal') as FontSize;
  return {
    theme: ['light', 'dark', 'system'].includes(theme) ? theme : 'light',
    locale,
    fontSize: ['small', 'normal', 'large'].includes(fontSize) ? fontSize : 'normal',
  };
}

export function writeLocalPreferences(storage: Storage, prefs: Partial<LocalPreferences>): void {
  if (prefs.theme) safeSet(storage, userKey(storage, 'theme'), prefs.theme);
  if (prefs.locale) safeSet(storage, LOCALE_KEY, prefs.locale);
  if (prefs.fontSize) safeSet(storage, userKey(storage, 'font_size'), prefs.fontSize);
}

/** Validate a theme value (Vue GeneralSettings.vue options). */
export function isValidTheme(v: string): v is ThemeMode { return ['light', 'dark', 'system'].includes(v); }
/** Validate a font size value. */
export function isValidFontSize(v: string): v is FontSize { return ['small', 'normal', 'large'].includes(v); }

let migratedForUser: string | null = null;

/**
 * Adopt legacy and anon preferences into the current user's namespace, then
 * remove the source keys (Vue preferenceStorage.migratePreferencesIntoUser).
 *
 * For each suffix, when the user key is absent, the first source with a
 * value wins: anon namespace → legacy un-namespaced key (`WeKnora_${suffix}`,
 * from earlier Vue branch versions) → React's previous flat keys
 * (`weknora-theme` / `weknora-font-size`). Source keys are always removed,
 * even when the target already exists, so the next user to log in cannot
 * inherit them. Idempotent per user per module lifetime; call
 * resetMigrationLatch() when the active user changes. Safe before login
 * (returns early for "anon").
 */
export function migratePreferencesIntoUser(storage: Storage): void {
  const userId = readUserIdFromStorage(storage);
  if (userId === 'anon') return;
  if (migratedForUser === userId) return;
  migratedForUser = userId;

  for (const suffix of PREFERENCE_SUFFIXES) {
    const target = `WeKnora_${userId}_${suffix}`;
    const targetExists = safeGet(storage, target) !== null;

    const anonKey = `WeKnora_anon_${suffix}`;
    const legacyKey = `WeKnora_${suffix}`;
    // React's previous flat keys before per-user namespacing.
    const reactFlatKey = suffix === 'theme' ? 'weknora-theme'
      : suffix === 'font_size' ? 'weknora-font-size'
      : null;

    if (!targetExists) {
      const value = safeGet(storage, anonKey)
        ?? safeGet(storage, legacyKey)
        ?? (reactFlatKey ? safeGet(storage, reactFlatKey) : null);
      if (value !== null) safeSet(storage, target, value);
    }

    // Always clean up source keys so subsequent users cannot inherit them.
    safeRemove(storage, anonKey);
    safeRemove(storage, legacyKey);
    if (reactFlatKey) safeRemove(storage, reactFlatKey);
  }
}

/** Resets the per-session migration latch (used when the active user changes). */
export function resetMigrationLatch(): void {
  migratedForUser = null;
}
