import assert from 'node:assert/strict';
import test from 'node:test';

import { apiKeyAccessMode, apiKeyValueDisplay, isFreshKeyVisible, type ApiKeyRow } from './apiKeys.ts';

// Vue baseline: frontend/src/components/ApiIntegrationSettings.vue —
// the table masks stored key values, labels the access mode from
// full_access/capabilities, and reveals a freshly created value once.

const row = (overrides: Partial<ApiKeyRow>): ApiKeyRow => ({
  id: 1, name: 'CI key', api_key: '', full_access: false,
  ...overrides,
});

test('masks stored key values like the Vue fingerprint column', () => {
  assert.equal(apiKeyValueDisplay(row({ api_key: 'wk-abcdef1234567890' }), false), 'wk-ab…7890');
  assert.equal(apiKeyValueDisplay(row({ api_key: 'short' }), false), 'short');
  assert.equal(apiKeyValueDisplay(row({ api_key: '' }), false), '(unavailable)');
});

test('freshly created keys reveal their value once', () => {
  assert.equal(apiKeyValueDisplay(row({ api_key: 'wk-abcdef1234567890' }), true), 'wk-abcdef1234567890');
  assert.equal(isFreshKeyVisible({ fresh: true, hasValue: true }), true);
  assert.equal(isFreshKeyVisible({ fresh: true, hasValue: false }), false);
  assert.equal(isFreshKeyVisible({ fresh: false, hasValue: true }), false);
});

test('access mode reflects full access or scoped capabilities', () => {
  assert.equal(apiKeyAccessMode(row({ full_access: true })), 'Full access');
  assert.equal(apiKeyAccessMode(row({ full_access: false, capabilities: ['retrieve', 'chat'] })), 'retrieve, chat');
  assert.equal(apiKeyAccessMode(row({ full_access: false })), 'Scoped');
});
