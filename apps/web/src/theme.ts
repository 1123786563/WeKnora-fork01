// Theme application for the React app — Vue useTheme.ts parity.
// Reads the stored preference (local-preferences) and applies the
// theme-mode attribute to <html> on startup, on weknora:theme-changed,
// and on OS scheme changes while mode is system.
import { applyThemeToDocument, type ThemeDocumentLike } from '@weknora/domain/settings/theme';
import { readLocalPreferences, migratePreferencesIntoUser } from '@weknora/domain/settings/local-preferences';

export function initTheme(): void {
  const apply = (): void => {
    let mode: string | null = null;
    try {
      // Vue preferenceStorage parity: adopt legacy/anon keys into the active
      // user's namespace before reading (idempotent; latch-guarded per user).
      migratePreferencesIntoUser(window.localStorage);
      mode = readLocalPreferences(window.localStorage).theme;
    } catch { mode = 'light'; }
    // The domain adapter intentionally accepts only the small DOM surface it
    // mutates; keep the browser's richer CSSStyleDeclaration at this boundary.
    applyThemeToDocument(document as unknown as ThemeDocumentLike, mode === 'dark' ? 'dark' : mode === 'system' ? 'system' : 'light', window.matchMedia('(prefers-color-scheme: dark)').matches);
  };
  apply();
  window.addEventListener('weknora:theme-changed', apply);
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', apply);
}
