import assert from 'node:assert/strict';
import test from 'node:test';
import { chatAppStateAction } from './appstate.ts';

test('backgrounding an active chat stream requests local cancellation', () => {
  assert.equal(chatAppStateAction('background', true, true), 'abort');
});

test('returning active with an authenticated session and no stream requests recovery', () => {
  assert.equal(chatAppStateAction('active', true, false), 'resume');
});

test('active state never starts a second recovery while a stream is still owned', () => {
  assert.equal(chatAppStateAction('active', true, true), 'ignore');
  assert.equal(chatAppStateAction('active', false, false), 'ignore');
});
