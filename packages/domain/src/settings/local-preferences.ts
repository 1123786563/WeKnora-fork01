// Port of Vue GeneralSettings.vue local preference handling: theme, language,
// sans/mono font, font size — all client-side (localStorage + CSS variables).
export type ThemeMode = 'light' | 'dark' | 'system';
export type FontSize = 'small' | 'normal' | 'large';

const THEME_KEY = 'weknora-theme';
const LOCALE_KEY = 'locale';
const FONT_SIZE_KEY = 'weknora-font-size';

export interface LocalPreferences {
  theme: ThemeMode;
  locale: string;
  fontSize: FontSize;
}

type Storage = { getItem(k: string): string | null; setItem(k: string, v: string): void };

export function readLocalPreferences(storage: Storage): LocalPreferences {
  // Vue useTheme.ts:14-15 — a missing or invalid stored theme resolves to
  // 'light'; 'system' is only an explicit user choice.
  const theme = (storage.getItem(THEME_KEY) ?? 'light') as ThemeMode;
  const locale = storage.getItem(LOCALE_KEY) ?? 'zh-CN';
  const fontSize = (storage.getItem(FONT_SIZE_KEY) ?? 'normal') as FontSize;
  return {
    theme: ['light', 'dark', 'system'].includes(theme) ? theme : 'light',
    locale,
    fontSize: ['small', 'normal', 'large'].includes(fontSize) ? fontSize : 'normal',
  };
}

export function writeLocalPreferences(storage: Storage, prefs: Partial<LocalPreferences>): void {
  if (prefs.theme) storage.setItem(THEME_KEY, prefs.theme);
  if (prefs.locale) storage.setItem(LOCALE_KEY, prefs.locale);
  if (prefs.fontSize) storage.setItem(FONT_SIZE_KEY, prefs.fontSize);
}

/** Validate a theme value (Vue GeneralSettings.vue options). */
export function isValidTheme(v: string): v is ThemeMode { return ['light', 'dark', 'system'].includes(v); }
/** Validate a font size value. */
export function isValidFontSize(v: string): v is FontSize { return ['small', 'normal', 'large'].includes(v); }
