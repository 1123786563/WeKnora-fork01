import assert from 'node:assert/strict';
import test from 'node:test';
import { designTokens } from './tokens.ts';

test('exposes semantic focus, disabled, and typography tokens for platform primitives', () => {
  assert.equal(designTokens.state.focusRing, '3px solid rgb(46 109 230 / 35%)');
  assert.equal(designTokens.state.disabledOpacity, 0.6);
  assert.equal(designTokens.typography.body.fontFamily, 'Inter, ui-sans-serif, system-ui, sans-serif');
});
