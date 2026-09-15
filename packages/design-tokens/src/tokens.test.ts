import assert from 'node:assert/strict';
import test from 'node:test';
import { designTokens } from './tokens.ts';

test('exposes semantic focus, disabled, and typography tokens for platform primitives', () => {
  assert.equal(designTokens.state.focusRing, '3px solid rgb(46 109 230 / 35%)');
  assert.equal(designTokens.state.disabledOpacity, 0.6);
  assert.equal(designTokens.typography.body.fontFamily, '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif');
});

test('keeps shared primitives aligned with the Vue control and surface palette', () => {
  assert.equal(designTokens.color.accent, '#07c05f');
  assert.equal(designTokens.color.accentHover, '#08dd6e');
  assert.equal(designTokens.color.accentActive, '#06b04d');
  assert.equal(designTokens.color.surfaceMuted, '#f3f3f3');
  assert.equal(designTokens.color.lineControl, '#dcdcdc');
  assert.equal(designTokens.radius.field, '3px');
  assert.equal(designTokens.radius.control, '6px');
  assert.equal(designTokens.typography.mono.fontFamily, 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace');
});
