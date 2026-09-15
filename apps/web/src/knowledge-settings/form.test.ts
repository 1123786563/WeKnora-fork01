import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildKnowledgeSettingsPayload,
  defaultKnowledgeSettingsFormState,
  normalizeKnowledgeSettingsFormState,
  validateKnowledgeSettingsFormState,
} from './form.ts';

test('defaults all Vue editor sections without sharing mutable values', () => {
  const first = defaultKnowledgeSettingsFormState();
  const second = defaultKnowledgeSettingsFormState();

  assert.deepEqual(first.processing.multimodal, { enabled: false, vllmModelId: '', descriptionLanguage: '', customInstructions: '' });
  assert.deepEqual(first.processing.asr, { enabled: false, modelId: '', language: '' });
  assert.deepEqual(first.wiki, { maxPagesPerIngest: 0, extractionGranularity: 'standard', contentInstructions: '', extractionInstructions: '' });
  assert.deepEqual(first.graph, { enabled: false, text: '', tags: [], nodes: [], relations: [], customInstructions: '' });

  first.processing.languages.push('zh');
  (first.graph.nodes as Array<{ name: string; attributes: string[] }>).push({ name: 'Person', attributes: [] });
  assert.deepEqual(second.processing.languages, []);
  assert.deepEqual(second.graph.nodes, []);
});

test('normalizes API snake_case values while preserving explicit false and zero', () => {
  const state = normalizeKnowledgeSettingsFormState({
    type: 'document',
    name: 'Docs',
    summary_model_id: 'llm-1',
    embedding_model_id: '',
    vlm_config: { enabled: false, model_id: 'ignored', description_language: 'zh', custom_instructions: 'describe' },
    asr_config: { enabled: false, model_id: 'ignored', language: 'en' },
    wiki_config: { max_pages_per_ingest: 0, extraction_granularity: 'focused', content_instructions: 'content', extraction_instructions: 'extract' },
    indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true, graph_enabled: false },
    extract_config: { enabled: false, text: 'sample', tags: ['Person'], nodes: [{ name: 'User', attributes: [] }], relations: [], custom_instructions: '' },
  });

  assert.equal(state.model.llmModelId, 'llm-1');
  assert.equal(state.model.embeddingModelId, '');
  assert.equal(state.processing.multimodal.enabled, false);
  assert.equal(state.processing.asr.enabled, false);
  assert.equal(state.wiki.maxPagesPerIngest, 0);
  assert.deepEqual(state.indexing, { vectorEnabled: false, keywordEnabled: false, wikiEnabled: true, graphEnabled: false });
  assert.equal(state.graph.enabled, false);
});

test('maps FAQ, multimodal, ASR, wiki, graph, and indexing fields exactly on create', () => {
  const state = defaultKnowledgeSettingsFormState('document');
  state.name = 'Docs';
  state.description = 'Description';
  state.model = { embeddingModelId: 'embed-1', llmModelId: 'llm-1', wikiSynthesisModelId: 'wiki-llm' };
  state.processing.multimodal = { enabled: true, vllmModelId: 'vlm-1', descriptionLanguage: 'zh', customInstructions: 'caption' };
  state.processing.asr = { enabled: true, modelId: 'asr-1', language: 'zh' };
  state.wiki = { maxPagesPerIngest: 20, extractionGranularity: 'exhaustive', contentInstructions: 'content', extractionInstructions: 'extract' };
  state.indexing = { vectorEnabled: true, keywordEnabled: false, wikiEnabled: true, graphEnabled: true };
  state.graph = { enabled: true, text: 'example', tags: ['Person'], nodes: [{ name: 'User', attributes: ['email'] }], relations: [{ node1: 'User', type: 'owns', node2: 'Account' }], customInstructions: 'extract carefully' };
  state.storage.backendId = 'storage-1';
  state.vectorStore.id = 'store-1';

  assert.deepEqual(buildKnowledgeSettingsPayload(state, 'create'), {
    name: 'Docs', description: 'Description', type: 'document',
    chunking_config: {
      chunk_size: 512, chunk_overlap: 80, separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
      enable_parent_child: true, parent_chunk_size: 4096, child_chunk_size: 384,
      strategy: 'auto', token_limit: 0, languages: [], table_metadata_instructions: '',
    },
    embedding_model_id: 'embed-1', summary_model_id: 'llm-1', vector_store_id: 'store-1', storage_backend_id: 'storage-1',
    vlm_config: { enabled: true, model_id: 'vlm-1', description_language: 'zh', custom_instructions: 'caption' },
    asr_config: { enabled: true, model_id: 'asr-1', language: 'zh' },
    storage_provider_config: { provider: 'local' }, storage_config: { provider: 'local' },
    question_generation_config: { enabled: true, question_count: 3, custom_instructions: '' },
    auto_tag_config: { enabled: false, model_id: '', max_tags: 3, skip_if_tagged: true },
    wiki_config: { synthesis_model_id: 'wiki-llm', max_pages_per_ingest: 20, extraction_granularity: 'exhaustive', content_instructions: 'content', extraction_instructions: 'extract' },
    indexing_strategy: { vector_enabled: true, keyword_enabled: false, wiki_enabled: true, graph_enabled: true },
    extract_config: { enabled: true, text: 'example', tags: ['Person'], nodes: [{ name: 'User', attributes: ['email'] }], relations: [{ node1: 'User', type: 'owns', node2: 'Account' }], custom_instructions: 'extract carefully' },
  });
});

test('update keeps FAQ config in the base update and full processing config separate', () => {
  const state = defaultKnowledgeSettingsFormState('faq');
  state.name = 'FAQ';
  state.model.llmModelId = 'llm-1';
  state.faq = { indexMode: 'question_answer', questionIndexMode: 'combined' };
  state.processing.multimodal.enabled = true;
  state.processing.multimodal.vllmModelId = 'vlm-1';
  const payload = buildKnowledgeSettingsPayload(state, 'update');

  assert.deepEqual(payload.base, { name: 'FAQ', description: '', config: { faq_config: { index_mode: 'question_answer', question_index_mode: 'combined' } } });
  assert.deepEqual(payload.config.vlm_config, { enabled: true, model_id: 'vlm-1', description_language: '', custom_instructions: '' });
  assert.equal('indexing_strategy' in payload.base.config, false);
  assert.equal('wiki_config' in payload.base.config, false);
});

test('validation only requires embedding when vector retrieval is enabled', () => {
  const state = defaultKnowledgeSettingsFormState();
  state.name = 'Wiki';
  state.model.llmModelId = 'llm-1';
  state.indexing = { vectorEnabled: false, keywordEnabled: false, wikiEnabled: true, graphEnabled: false };
  assert.deepEqual(validateKnowledgeSettingsFormState(state), []);

  state.processing.multimodal.enabled = true;
  assert.deepEqual(validateKnowledgeSettingsFormState(state), [{ field: 'vlm_model_id', section: 'multimodal', message: 'Multimodal model is required' }]);
});
