import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const theme = readFileSync(new URL('./theme.css', import.meta.url), 'utf8');

test('shared Tailwind tokens expose the Vue-compatible brand and surface aliases', () => {
  assert.match(theme, /--color-brand:\s*var\(--wk-color-brand/);
  assert.match(theme, /--color-brand-hover:\s*var\(--wk-color-brand-hover/);
  assert.match(theme, /--color-brand-active:\s*var\(--wk-color-brand-active/);
  assert.match(theme, /--color-brand-light:\s*var\(--wk-color-brand-light/);
  assert.match(theme, /--color-text:\s*var\(--wk-color-text/);
  assert.match(theme, /--color-text-secondary:\s*var\(--wk-color-text-secondary/);
  assert.match(theme, /--color-text-placeholder:\s*var\(--wk-color-text-placeholder/);
  assert.match(theme, /--color-text-disabled:\s*var\(--wk-color-text-disabled/);
  assert.match(theme, /--color-page:\s*var\(--wk-color-page/);
  assert.match(theme, /--color-surface-hover:\s*var\(--wk-color-surface-hover/);
  assert.match(theme, /--color-surface-active:\s*var\(--wk-color-surface-active/);
  assert.match(theme, /--color-settings-panel:\s*var\(--wk-color-settings-panel/);
  assert.match(theme, /--color-border:\s*var\(--wk-color-border/);
  assert.match(theme, /--color-component:\s*var\(--wk-color-component/);
  assert.match(theme, /--color-error:\s*var\(--wk-color-error/);
  assert.match(theme, /--color-error-light:\s*var\(--wk-color-error-light/);
});

test('shared Tailwind tokens keep Vue light values in the canonical design token source', () => {
  const designTokens = readFileSync(new URL('../../../packages/design-tokens/src/styles.css', import.meta.url), 'utf8');

  assert.match(designTokens, /--wk-color-brand: #07c05f/);
  assert.match(designTokens, /--wk-color-page: #eeeeee/);
  assert.match(designTokens, /--wk-color-surface-hover: #f3f3f3/);
  assert.match(designTokens, /--wk-color-component: #e7e7e7/);
});
