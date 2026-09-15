import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildKnowledgeBaseEditorPayload,
  defaultKnowledgeBaseEditorConfig,
  hydrateKnowledgeBaseEditorConfig,
} from './editor-config.ts';

test('returns Vue-aligned defaults with isolated nested values', () => {
  const first = defaultKnowledgeBaseEditorConfig('document');
  const second = defaultKnowledgeBaseEditorConfig('document');

  assert.deepEqual(first.chunking, {
    chunkSize: 512,
    chunkOverlap: 80,
    separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
    parserEngineRules: undefined,
    enableParentChild: true,
    parentChunkSize: 4096,
    childChunkSize: 384,
    strategy: 'auto',
    tokenLimit: 0,
    languages: [],
    tableMetadataInstructions: '',
  });
  assert.equal(first.processing.multimodal.enabled, false);
  assert.equal(first.processing.asr.enabled, false);
  assert.equal(first.processing.questionGeneration.enabled, true);
  assert.equal(first.processing.autoTag.skipIfTagged, true);
  assert.equal(first.faq.indexMode, 'question_only');
  assert.equal(first.processing.wiki.extractionGranularity, 'standard');
  assert.equal(first.storage.provider, 'local');

  first.chunking.separators.push('custom');
  first.processing.languages.push('zh');
  first.processing.graph.nodes.push({ name: 'Person', attributes: ['name'] });
  assert.deepEqual(second.chunking.separators, ['\n\n', '\n', '。', '！', '？', ';', '；']);
  assert.deepEqual(second.processing.languages, []);
  assert.deepEqual(second.processing.graph.nodes, []);
});

test('hydrates every Vue snake_case field and preserves explicit falsy values', () => {
  const state = hydrateKnowledgeBaseEditorConfig({
    type: 'faq',
    name: 'Support',
    description: 'Answers',
    faq_config: { index_mode: 'question_answer', question_index_mode: 'combined' },
    chunking_config: {
      chunk_size: 0,
      chunk_overlap: 0,
      separators: [],
      parser_engine_rules: [{ parser: 'mineru', fileTypes: ['pdf'] }],
      enable_parent_child: false,
      parent_chunk_size: 0,
      child_chunk_size: 0,
      strategy: '',
      token_limit: 0,
      languages: [],
      table_metadata_instructions: '',
    },
    embedding_model_id: 'embed-1',
    summary_model_id: 'llm-1',
    vlm_config: { enabled: false, model_id: 'ignored', description_language: 'zh', custom_instructions: 'x' },
    asr_config: { enabled: true, model_id: 'asr-1', language: 'zh-CN' },
    extract_config: {
      enabled: false,
      text: '',
      tags: [],
      nodes: [{ name: 'Person', attributes: [] }],
      relations: [],
      custom_instructions: '',
    },
    question_generation_config: { enabled: false, question_count: 0, custom_instructions: '' },
    auto_tag_config: { enabled: false, model_id: '', max_tags: 0, skip_if_tagged: false },
    wiki_config: {
      synthesis_model_id: 'wiki-1',
      max_pages_per_ingest: 0,
      extraction_granularity: 'unexpected',
      content_instructions: '',
      extraction_instructions: '',
    },
    indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true, graph_enabled: false },
    storage_backend_id: 'backend-1',
    storage_provider_config: { provider: 's3' },
    storage_config: { provider: 'local' },
    vector_store_id: 'store-1',
  });

  assert.equal(state.type, 'faq');
  assert.equal(state.name, 'Support');
  assert.equal(state.chunking.chunkSize, 0);
  assert.equal(state.chunking.enableParentChild, false);
  assert.deepEqual(state.chunking.separators, []);
  assert.equal(state.processing.multimodal.modelId, '');
  assert.equal(state.processing.asr.enabled, true);
  assert.equal(state.processing.questionGeneration.questionCount, 0);
  assert.equal(state.processing.autoTag.maxTags, 0);
  assert.equal(state.processing.autoTag.skipIfTagged, false);
  assert.equal(state.processing.wiki.extractionGranularity, 'standard');
  assert.deepEqual(state.indexing, { vectorEnabled: false, keywordEnabled: false, wikiEnabled: true, graphEnabled: false });
  assert.deepEqual(state.storage, { backendId: 'backend-1', provider: 's3' });
});

test('builds create payload with backend snake_case contracts', () => {
  const state = defaultKnowledgeBaseEditorConfig('document');
  state.name = 'Docs';
  state.description = 'Reference';
  state.model.embeddingModelId = 'embed-1';
  state.model.llmModelId = 'llm-1';
  state.storage.backendId = 'backend-1';
  state.storage.provider = 's3';
  state.processing.multimodal.enabled = true;
  state.processing.multimodal.modelId = 'vlm-1';
  state.processing.asr.enabled = true;
  state.processing.asr.modelId = 'asr-1';
  state.processing.graph.enabled = true;
  state.processing.graph.tags = ['Person'];
  state.processing.wiki.enabled = true;
  state.indexing.graphEnabled = true;
  state.indexing.wikiEnabled = true;
  state.processing.questionGeneration.customInstructions = 'Generate concise questions';

  assert.deepEqual(buildKnowledgeBaseEditorPayload(state, 'create'), {
    name: 'Docs',
    description: 'Reference',
    type: 'document',
    chunking_config: {
      chunk_size: 512,
      chunk_overlap: 80,
      separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
      enable_parent_child: true,
      parent_chunk_size: 4096,
      child_chunk_size: 384,
      strategy: 'auto',
      token_limit: 0,
      languages: [],
      table_metadata_instructions: '',
    },
    embedding_model_id: 'embed-1',
    summary_model_id: 'llm-1',
    storage_backend_id: 'backend-1',
    storage_provider_config: { provider: 's3' },
    storage_config: { provider: 's3' },
    vlm_config: { enabled: true, model_id: 'vlm-1', description_language: '', custom_instructions: '' },
    asr_config: { enabled: true, model_id: 'asr-1', language: '' },
    question_generation_config: { enabled: true, question_count: 3, custom_instructions: 'Generate concise questions' },
    auto_tag_config: { enabled: false, model_id: '', max_tags: 3, skip_if_tagged: true },
    wiki_config: {
      synthesis_model_id: '', max_pages_per_ingest: 0, extraction_granularity: 'standard',
      content_instructions: '', extraction_instructions: '',
    },
    indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: true, graph_enabled: true },
    extract_config: {
      enabled: true, text: '', tags: ['Person'], nodes: [], relations: [], custom_instructions: '',
    },
  });
});

test('builds update payload with base metadata and config-only editor fields', () => {
  const state = defaultKnowledgeBaseEditorConfig('faq');
  state.name = 'FAQ';
  state.description = 'Updated';
  state.model.llmModelId = 'llm-1';
  state.faq.indexMode = 'question_answer';
  state.storage.backendId = 'backend-1';

  const payload = buildKnowledgeBaseEditorPayload(state, 'update');
  assert.deepEqual(payload, {
    base: {
      name: 'FAQ',
      description: 'Updated',
      config: { faq_config: { index_mode: 'question_answer', question_index_mode: 'separate' } },
    },
    config: {
      embedding_model_id: '',
      summary_model_id: 'llm-1',
      chunking_config: {
        chunk_size: 512, chunk_overlap: 80, separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
        enable_parent_child: true, parent_chunk_size: 4096,
        child_chunk_size: 384, strategy: 'auto', token_limit: 0, languages: [], table_metadata_instructions: '',
      },
      vlm_config: { enabled: false, model_id: '', description_language: '', custom_instructions: '' },
      asr_config: { enabled: false, model_id: '', language: '' },
      storage_provider_config: { provider: 'local' },
      storage_config: { provider: 'local' },
      question_generation_config: { enabled: true, question_count: 3, custom_instructions: '' },
      auto_tag_config: { enabled: false, model_id: '', max_tags: 3, skip_if_tagged: true },
      extract_config: { enabled: false, text: '', tags: [], nodes: [], relations: [], custom_instructions: '' },
    },
  });
  assert.equal('storage_backend_id' in payload.config, false);
  assert.equal('faq_config' in payload.config, false);
  assert.equal('indexing_strategy' in payload.base.config, false);
});
