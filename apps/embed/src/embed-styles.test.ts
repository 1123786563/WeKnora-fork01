import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const stylesPath = fileURLToPath(new URL('./styles.css', import.meta.url));
const utilityStylesPath = fileURLToPath(new URL('../../web/src/embed/embed-u.css', import.meta.url));
const styles = readFileSync(stylesPath, 'utf8');
const utilityStyles = readFileSync(utilityStylesPath, 'utf8');

function ruleFor(source: string, selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = source.match(new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`));
  assert.ok(match, `expected CSS rule for ${selector}`);
  return match[1];
}

test('embed migrated hover underline preserves its offset', () => {
  assert.match(ruleFor(utilityStyles, '.wk-emb-34:hover'), /text-underline-offset:\s*2px/);
});

test('embed primary button colors use design token fallbacks', () => {
  assert.match(ruleFor(styles, '.embed-btn--primary'), /--wk-color-brand\s*,\s*#07c05f/);
  assert.match(ruleFor(styles, '.embed-btn--primary:hover:not(:disabled)'), /--wk-color-brand-hover\s*,\s*#0a8f4c/);
});

test('dark mode text buttons inherit readable foreground and subtle current color surface', () => {
  const darkTextButton = ruleFor(styles, ':root[data-theme="dark"] .embed-btn--text');
  assert.match(darkTextButton, /color:\s*inherit/);
  assert.match(darkTextButton, /background-color:\s*color-mix\(in srgb, currentColor [^,]+, transparent\)/);
});
