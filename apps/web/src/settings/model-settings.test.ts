import assert from 'node:assert/strict';
import test from 'node:test';

import { modelPayload, modelType, validateModelDraft, type ModelDraft } from './model-settings.ts';

const baseDraft: ModelDraft = {
  name: 'text-embedding-3-small', displayName: '', type: 'embedding', source: 'remote', provider: 'openai',
  baseUrl: 'https://api.openai.com/v1', dimension: 1536, supportsVision: false, contextWindow: '', maxConcurrency: '',
  thinkingControl: '', customHeaders: {}, apiKey: '', appSecret: '',
};

test('model type and payload preserve Vue model fields and isolate credentials', () => {
  assert.equal(modelType({ type: 'EmbeddingModel' }), 'embedding');
  assert.deepEqual(modelPayload(baseDraft), {
    name: 'text-embedding-3-small', display_name: 'text-embedding-3-small', description: '', type: 'Embedding', source: 'remote',
    parameters: { base_url: 'https://api.openai.com/v1', provider: 'openai', embedding_parameters: { dimension: 1536, truncate_prompt_tokens: 0, supports_dimension_override: false } },
  });
});

test('model validation mirrors Vue required, URL, and embedding dimension rules', () => {
  assert.deepEqual(validateModelDraft({ ...baseDraft, name: '', baseUrl: '' }), ['modelNameRequired', 'baseUrlRequired']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, baseUrl: 'not-a-url' }), ['baseUrlInvalid']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, dimension: 64 }), ['dimensionInvalid']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, dimension: 1536 }), []);
});
