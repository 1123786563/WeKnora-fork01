// Font-size application for the React app — Vue useFont.ts applyFont parity.
//
// Vue applies the persisted font size as a CSS `zoom` on <html> so the
// multiplier reaches every element — including the ~1000+ hard-coded
// `font-size: NNpx` rules (frontend/src/composables/useFont.ts:216-238; the
// comment there explains why a calc()-driven CSS variable only resized parts
// of the UI). The previous React implementation only set `--wk-font-scale`,
// a variable with no consumers, so the 字号 segmented control had no visual
// effect (px2-settings-general-fontradio baseline 25.519%: Vue zoomed the
// whole document at 1.125 while React changed nothing).
//
// applyFont always rewrites the zoom (even `normal` = 1) and Vue main.ts:27
// runs initFont() before mount so a reload keeps the size — mirror both.
import { readLocalPreferences, migratePreferencesIntoUser } from '@weknora/domain/settings/local-preferences';

/** Vue useFont.FONT_SCALES (useFont.ts:106-110) verbatim. */
export const FONT_SCALES: Record<string, number> = { small: 0.875, normal: 1, large: 1.125 };

/** Apply a font size key as the document zoom (Vue useFont.applyFont scale branch). */
export function applyFontSizeZoom(size: string): void {
  const scale = FONT_SCALES[size] ?? FONT_SCALES.normal!;
  document.documentElement.style.setProperty('zoom', String(scale));
}

/** Call once at app boot to apply the persisted font size before mount (Vue initFont). */
export function initFont(): void {
  let fontSize = 'normal';
  try {
    // Vue preferenceStorage parity: adopt legacy/anon keys into the active
    // user's namespace before reading (idempotent; latch-guarded per user).
    migratePreferencesIntoUser(window.localStorage);
    fontSize = readLocalPreferences(window.localStorage).fontSize;
  } catch { fontSize = 'normal'; }
  applyFontSizeZoom(fontSize);
}
