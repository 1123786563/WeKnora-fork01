import assert from 'node:assert/strict';
import test from 'node:test';
import { tokens, tokenCss } from './index.ts';

test('exports Vue-authoritative semantic tokens', () => {
  assert.equal(tokens.color.brand, '#07c05f');
  assert.equal(tokens.color.brandHover, '#08dd6e');
  assert.equal(tokens.font.size.body, '14px');
  assert.equal(tokens.font.lineHeight.body, '20px');
  assert.equal(tokens.radius.control, '6px');
  assert.equal(tokens.overlay.menuMinWidth, '148px');
  assert.equal(tokens.overlay.anchoredZIndex, 3500);
  assert.match(tokenCss, /--wk-color-brand: #07c05f/);
  assert.match(tokenCss, /--wk-overlay-menu-min-width: 148px/);
});
