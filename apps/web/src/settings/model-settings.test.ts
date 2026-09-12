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
    parameters: { base_url: 'https://api.openai.com/v1', provider: 'openai', embedding_parameters: { dimension: 1536 } },
  });
});

test('model payload uses the exact backend type vocabulary and preserves unknown parameters', () => {
  for (const [type, backend] of [['chat', 'KnowledgeQA'], ['embedding', 'Embedding'], ['rerank', 'Rerank'], ['vllm', 'VLLM'], ['asr', 'ASR']] as const) {
    const payload = modelPayload({ ...baseDraft, type, source: type === 'rerank' ? 'local' : 'remote', originalParameters: { max_output_tokens: 2048 } });
    assert.equal(payload.type, backend);
    assert.equal((payload.parameters as Record<string, unknown>).max_output_tokens, 2048);
    if (type === 'rerank') assert.equal(payload.source, 'remote');
  }
});

test('model validation mirrors Vue required, URL, and embedding dimension rules', () => {
  assert.deepEqual(validateModelDraft({ ...baseDraft, name: '', baseUrl: '' }), ['modelNameRequired', 'baseUrlRequired']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, baseUrl: 'not-a-url' }), ['baseUrlInvalid']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, dimension: 64 }), ['dimensionInvalid']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, dimension: 1536 }), []);
});
