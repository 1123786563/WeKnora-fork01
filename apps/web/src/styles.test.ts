import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const styles = readFileSync(new URL('./styles.css', import.meta.url), 'utf8');
const tokenStyles = readFileSync(new URL('../../../packages/design-tokens/src/styles.css', import.meta.url), 'utf8');
const compiledStyles = `${styles}\n${tokenStyles}`;

test('web theme maps Vue dark mode and shadcn semantic variables', () => {
  assert.match(compiledStyles, /:root\[theme-mode=['"]dark['"]\]/);
  assert.match(compiledStyles, /--wk-color-page: #181818/);
  assert.match(compiledStyles, /--wk-color-panel: #181818/);
  assert.match(compiledStyles, /--background: var\(--wk-color-page\)/);
  assert.match(compiledStyles, /--popover: var\(--wk-color-surface\)/);
  assert.match(compiledStyles, /--card: var\(--wk-color-surface\)/);
});

test('web overlay layers use the Vue Portal z-index contracts', () => {
  assert.match(styles, /\.wks-overlay\s*\{[\s\S]*z-index: var\(--wk-overlay-settings-z, 1100\)/);
  assert.match(compiledStyles, /--wk-overlay-dialog-z: 3000/);
  assert.match(compiledStyles, /--wk-overlay-anchored-z: 3500/);
});
