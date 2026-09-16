import assert from 'node:assert/strict';
import test from 'node:test';

import { actionControls, authorizationStatus } from './model.ts';

test('uses the Vue app action lifecycle for approval controls', () => {
  assert.deepEqual(actionControls('awaiting_approval', true), { approve: true, execute: false });
  assert.deepEqual(actionControls('awaiting_approval', false), { approve: false, execute: false });
  assert.deepEqual(actionControls('authorized', true), { approve: false, execute: true });
  assert.deepEqual(actionControls('succeeded', true), { approve: false, execute: false });
});

test('stops authorization polling for terminal and unknown states', () => {
  assert.equal(authorizationStatus('pending').poll, true);
  assert.equal(authorizationStatus('authorizing').poll, true);
  assert.equal(authorizationStatus('verifying').poll, true);
  assert.equal(authorizationStatus('active').poll, false);
  assert.equal(authorizationStatus('expired').poll, false);
  assert.equal(authorizationStatus('provider_failed').poll, false);
});
