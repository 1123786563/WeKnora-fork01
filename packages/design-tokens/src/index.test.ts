import assert from 'node:assert/strict';
import test from 'node:test';
import { tokens, tokenCss } from './index.ts';

test('exports Vue-authoritative semantic tokens', () => {
  assert.equal(tokens.color.brand, '#07c05f');
  assert.equal(tokens.color.brandHover, '#08dd6e');
  assert.equal(tokens.color.panel, '#ffffff');
  assert.equal(tokens.color.settingsPanel, '#f9f9f9');
  assert.equal(tokens.color.dark.panel, '#181818');
  assert.equal(tokens.color.dark.text, 'rgba(255, 255, 255, 0.9)');
  assert.equal(tokens.font.size.body, '14px');
  assert.match(tokens.font.family, /"Hiragino Sans GB"/);
  assert.match(tokens.font.mono, /"SF Mono"/);
  assert.equal(tokens.font.lineHeight.body, '20px');
  assert.equal(tokens.radius.control, '6px');
  assert.equal(tokens.overlay.menuMinWidth, '148px');
  assert.equal(tokens.overlay.anchoredZIndex, 3500);
  assert.equal(tokens.overlay.dialogZIndex, 3000);
  assert.match(tokenCss, /--wk-color-brand: #07c05f/);
  assert.match(tokenCss, /--wk-color-panel: #ffffff/);
  assert.match(tokenCss, /--wk-color-panel: #181818/);
  assert.match(tokenCss, /--wk-shadow-panel:/);
  assert.match(tokenCss, /--background: var\(--wk-color-page\)/);
  assert.match(tokenCss, /--popover: var\(--wk-color-surface\)/);
  assert.match(tokenCss, /--wk-overlay-menu-min-width: 148px/);
});
