// CFT-S00-T003: the craft scope must INHERIT the brand sources — blue
// brand/navigation, green create action — through semantic aliases layered on
// the existing design-tokens/theme variables, never by repainting the global
// palette. These tests pin the three acceptance assertions:
//   1. blue brand stays #2e6de6 and green action stays #07c05f (sources)
//   2. craft aliases live under the .wk-craft scope only — the global
//      :root primary is untouched
//   3. the small-size green button foreground keeps >= 4.5:1 contrast
//      (WCAG relative-luminance formula, computed, not eyeballed)
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { designTokens } from './tokens.ts';

const pkgRoot = fileURLToPath(new URL('.', import.meta.url));
const themeCss = readFileSync(new URL('../../ui/src/theme.css', import.meta.url), 'utf8');
const craftCss = readFileSync(new URL('../../views/src/craft/craft.css', import.meta.url), 'utf8');

/** WCAG 2.x relative luminance + contrast ratio. */
function contrast(fg: string, bg: string): number {
  const channel = (hex: string): [number, number, number] => {
    const v = hex.replace('#', '');
    return [parseInt(v.slice(0, 2), 16), parseInt(v.slice(2, 4), 16), parseInt(v.slice(4, 6), 16)];
  };
  const lum = (hex: string): number => {
    const [r, g, b] = channel(hex).map((c) => {
      const s = c / 255;
      return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  const [l1, l2] = [lum(fg), lum(bg)].sort((a, b) => b - a);
  return (l1 + 0.05) / (l2 + 0.05);
}

test('source tokens keep the WeKnora blue brand and green action', () => {
  assert.equal(designTokens.color.primary, '#2e6de6');
  assert.equal(designTokens.color.accent, '#07c05f');
  assert.equal(designTokens.color.accentHover, '#08dd6e');
  assert.equal(designTokens.color.accentActive, '#06b04d');
  assert.equal(designTokens.color.canvas, '#f7f9fc');
});

test('.wk-craft aliases exist and never repaint the global :root', () => {
  const scopeStart = themeCss.indexOf('.wk-craft {');
  assert.ok(scopeStart > 0, 'theme.css must define the .wk-craft scope block');
  const scopeEnd = themeCss.indexOf('}', scopeStart);
  const scope = themeCss.slice(scopeStart, scopeEnd);
  // every design-table alias resolves to an existing source variable
  for (const [alias, source] of [
    ['--craft-brand', 'var(--color-primary'],
    ['--craft-brand-strong', 'var(--color-primary-strong'],
    ['--craft-brand-soft', 'var(--color-surface-wash'],
    ['--craft-accent', 'var(--color-accent'],
    ['--craft-accent-hover', 'var(--color-accent-hover'],
    ['--craft-accent-active', 'var(--color-accent-active'],
    ['--craft-accent-soft', 'var(--color-brand-light'],
    ['--craft-canvas', 'var(--color-canvas'],
    ['--craft-surface', 'var(--color-surface'],
    ['--craft-ink', 'var(--color-ink'],
    ['--craft-muted', 'var(--color-muted'],
    ['--craft-secondary-text', 'var(--color-muted-strong'],
    ['--craft-line', 'var(--color-line'],
    ['--craft-line-soft', 'var(--color-line-soft'],
    ['--craft-line-strong', 'var(--color-line-strong'],
    ['--craft-success-text', 'var(--color-success-text'],
    ['--craft-danger', 'var(--color-danger'],
  ] as const) {
    assert.ok(scope.includes(`${alias}: ${source}`), `alias ${alias} must resolve ${source}`);
  }
  // the accessibility correction: green buttons use the dark ink foreground
  assert.ok(scope.includes('--craft-action-fg: var(--color-ink)'), 'green action foreground must be ink (#172033), not white');
  // the aliases must NOT leak into the global @theme source — the blue
  // brand keeps its single definition there and gains no craft override
  // (theme.css defines variables under Tailwind v4 @theme, not a :root block)
  const themeStart = themeCss.indexOf('@theme');
  const themeEnd = themeCss.indexOf('\n}', themeStart);
  const themeBlock = themeCss.slice(themeStart, themeEnd);
  assert.ok(themeBlock.includes('--color-primary: #2e6de6'), '@theme keeps the blue brand');
  assert.ok(!themeBlock.includes('--craft-'), 'no craft alias may be defined in the global @theme block');
  assert.equal((themeBlock.match(/--color-primary:/g) ?? []).length, 1, 'global primary stays single-sourced');
});

test('green small-size action foreground reaches at least 4.5:1', () => {
  const ink = '#172033'; // theme.css --color-ink, the craft action foreground
  assert.ok(contrast(ink, '#07c05f') >= 4.5, `ink on accent = ${contrast(ink, '#07c05f')}`);
  // the design's rejected alternative stays documented as insufficient:
  assert.ok(contrast('#ffffff', '#07c05f') < 4.5, 'white on accent is the contrast the design corrected away');
});

test('craft component styles carry no scattered brand literals', () => {
  for (const literal of ['#2e6de6', '#2E6DE6', '#07c05f', '#07C05F', '#0052d9', '#08dd6e', '#06b04d']) {
    assert.equal(craftCss.toLowerCase().includes(literal.toLowerCase()), false, `craft.css must reference --craft-* instead of ${literal}`);
  }
  assert.ok(craftCss.includes('var(--craft-'), 'craft.css consumes the scoped aliases');
});
