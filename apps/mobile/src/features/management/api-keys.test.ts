import assert from 'node:assert/strict';
import test from 'node:test';
import { canManageApiKeys, validateApiKeyDraft } from './api-keys.ts';

test('mobile API key management is limited to the owner role', () => {
  assert.equal(canManageApiKeys('owner'), true);
  assert.equal(canManageApiKeys('admin'), false);
  assert.equal(canManageApiKeys('contributor'), false);
  assert.equal(canManageApiKeys('viewer'), false);
});

test('mobile API key drafts reject blank names and empty capabilities', () => {
  assert.deepEqual(validateApiKeyDraft(' ', []), ['Name is required', 'Select at least one capability']);
  assert.deepEqual(validateApiKeyDraft('mobile-read', ['knowledge.read']), []);
});
