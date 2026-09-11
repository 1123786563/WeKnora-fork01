import assert from 'node:assert/strict';
import test from 'node:test';

import { configurationDraftFromRecord, credentialInput, savedConfigurationId } from './editor.ts';

test('creates an editable draft from redacted records without restoring secret placeholders', () => {
  const draft = configurationDraftFromRecord('models', {
    id: 'model-1', name: 'Model', description: 'Remote', type: 'KnowledgeQA', source: 'custom',
    parameters: { base_url: 'https://model.test', api_key: 'redacted-value' },
    credentials: { api_key: { configured: true } },
  });
  assert.equal(draft.id, 'model-1');
  assert.equal(draft.details, '{"base_url":"https://model.test"}');
  assert.equal(draft.apiKey, '');
});

test('only sends newly entered non-empty credentials to the dedicated subresource', () => {
  assert.deepEqual(credentialInput('models', { apiKey: ' new-key ', appSecret: '' }), { apiKey: 'new-key' });
  assert.deepEqual(credentialInput('mcp', { token: 'token-1', apiKey: '   ' }), { token: 'token-1' });
  assert.deepEqual(credentialInput('agents', { apiKey: 'unexpected' }), {});
});

test('uses the server id before writing credentials for a newly created resource', () => {
  assert.equal(savedConfigurationId(undefined, 'created-model'), 'created-model');
  assert.equal(savedConfigurationId('existing-model', 'ignored'), 'existing-model');
  assert.throws(() => savedConfigurationId(undefined, ''), /server id/);
});
