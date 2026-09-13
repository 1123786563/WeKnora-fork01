import { test } from 'node:test';
import assert from 'node:assert/strict';
import { applyThemeToDocument, effectiveTheme } from './theme.ts';

test('system mode resolves against the OS preference', () => {
  assert.equal(effectiveTheme('system', true), 'dark');
  assert.equal(effectiveTheme('system', false), 'light');
  assert.equal(effectiveTheme('dark', false), 'dark');
  assert.equal(effectiveTheme('light', true), 'light');
});

function fakeDoc() {
  const attrs: Record<string, string> = {};
  const style: Record<string, string> = {};
  return {
    attrs, style,
    documentElement: { setAttribute: (n: string, v: string) => { attrs[n] = v; }, style },
  };
}

test('applyThemeToDocument sets the theme-mode attribute and colorScheme', () => {
  const doc = fakeDoc();
  assert.equal(applyThemeToDocument(doc, 'dark', true), 'dark');
  assert.equal(doc.attrs['theme-mode'], 'dark');
  assert.equal(doc.documentElement.style.colorScheme, 'dark');
  assert.equal(applyThemeToDocument(doc, 'light', true), 'light');
  assert.equal(doc.attrs['theme-mode'], 'light');
});
