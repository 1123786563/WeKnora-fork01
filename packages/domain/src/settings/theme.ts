// Port of Vue frontend/src/composables/useTheme.ts theme resolution and
// application, kept framework-agnostic: the DOM and matchMedia are injected
// by the caller so the module stays testable in pure Node.

export type ThemeMode = 'light' | 'dark' | 'system';
export type EffectiveTheme = 'light' | 'dark';

/** Vue useTheme.ts:23-25 — OS prefers-color-scheme resolution. */
export function systemPrefersDark(matchMedia: (query: string) => { matches: boolean }): boolean {
  return matchMedia('(prefers-color-scheme: dark)').matches;
}

/** Vue useTheme.ts:64 — system mode resolves against the OS preference. */
export function effectiveTheme(mode: ThemeMode, systemPrefersDark: boolean): EffectiveTheme {
  if (mode === 'system') return systemPrefersDark ? 'dark' : 'light';
  return mode;
}

export interface ThemeDocumentLike {
  documentElement: { setAttribute(name: string, value: string): void; style: { colorScheme?: string } };
  body?: unknown;
}

/**
 * Vue useTheme.ts:67 — the whole tdesign theme keys off the theme-mode
 * attribute on <html>. React consumers must call this on startup and on
 * every weknora:theme-changed event.
 */
export function applyThemeToDocument(doc: ThemeDocumentLike, mode: ThemeMode, systemPrefersDark: boolean): EffectiveTheme {
  const effective = effectiveTheme(mode, systemPrefersDark);
  doc.documentElement.setAttribute('theme-mode', effective);
  doc.documentElement.style.colorScheme = effective;
  return effective;
}
