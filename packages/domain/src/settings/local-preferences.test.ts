import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readLocalPreferences, writeLocalPreferences, isValidTheme, isValidFontSize } from './local-preferences.ts';

function mem() {
  const m = new Map<string, string>();
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) };
}

test('defaults when storage is empty', () => {
  const p = readLocalPreferences(mem());
  assert.equal(p.theme, 'system');
  assert.equal(p.fontSize, 'normal');
});

test('round-trips theme, locale and font size', () => {
  const s = mem();
  writeLocalPreferences(s, { theme: 'dark', locale: 'ja-JP', fontSize: 'large' });
  const p = readLocalPreferences(s);
  assert.equal(p.theme, 'dark');
  assert.equal(p.locale, 'ja-JP');
  assert.equal(p.fontSize, 'large');
});

test('invalid values fall back to defaults', () => {
  const s = mem();
  s.setItem('weknora-theme', 'bogus');
  s.setItem('weknora-font-size', 'huge');
  const p = readLocalPreferences(s);
  assert.equal(p.theme, 'system');
  assert.equal(p.fontSize, 'normal');
});

test('isValidTheme and isValidFontSize gate correctly', () => {
  assert.ok(isValidTheme('light'));
  assert.ok(!isValidTheme('bogus'));
  assert.ok(isValidFontSize('large'));
  assert.ok(!isValidFontSize('huge'));
});
