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

test('exposes the Vue semantic palette and TDesign geometry used by Tailwind primitives', () => {
  assert.equal(designTokens.color.page, '#eeeeee');
  assert.equal(designTokens.color.sidebar, '#f9f9f9');
  assert.equal(designTokens.color.containerHover, '#f3f3f3');
  assert.equal(designTokens.color.textPrimary, 'rgba(0, 0, 0, 0.9)');
  assert.equal(designTokens.color.textSecondary, 'rgba(0, 0, 0, 0.6)');
  assert.equal(designTokens.color.textPlaceholder, 'rgba(0, 0, 0, 0.4)');
  assert.equal(designTokens.color.textDisabled, 'rgba(0, 0, 0, 0.26)');
  assert.equal(designTokens.radius.small, '2px');
  assert.equal(designTokens.radius.default, '3px');
  assert.equal(designTokens.radius.medium, '6px');
  assert.equal(designTokens.radius.extraLarge, '12px');
  assert.equal(designTokens.shadow.one, '0px 1px 10px rgba(0, 0, 0, 0.05), 0px 4px 5px rgba(0, 0, 0, 0.08), 0px 2px 4px -1px rgba(0, 0, 0, 0.12)');
});

test('maps shadcn semantic slots to Tailwind CSS variables instead of stock colors', () => {
  assert.equal(designTokens.tailwind.colors.background, 'var(--color-page, var(--color-canvas))');
  assert.equal(designTokens.tailwind.colors.foreground, 'var(--color-ink)');
  assert.equal(designTokens.tailwind.colors.card, 'var(--color-surface)');
  assert.equal(designTokens.tailwind.colors.primary, 'var(--color-accent)');
  assert.equal(designTokens.tailwind.colors.destructive, 'var(--color-danger)');
  assert.equal(designTokens.tailwind.borderRadius.md, 'var(--radius-control)');
  assert.equal(designTokens.shadcn.primary, 'accent');
  assert.equal(designTokens.shadcn.ring, 'focus');
});
