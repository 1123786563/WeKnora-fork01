import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildKnowledgeBasePayload,
  defaultKnowledgeBaseEditorState,
  getKnowledgeBaseSections,
  normalizeKnowledgeBaseEditorState,
  validateKnowledgeBaseEditorState,
  type KnowledgeBaseEditorState,
} from './editor.ts';

test('exposes the same section order for document and FAQ editors', () => {
  assert.deepEqual(
    getKnowledgeBaseSections('document').map((section) => section.key),
    ['basic', 'models', 'vectorStore', 'parser', 'multimodal', 'asr', 'storage', 'chunking', 'graph', 'advanced'],
  );
  assert.deepEqual(
    getKnowledgeBaseSections('faq').map((section) => section.key),
    ['basic', 'models', 'vectorStore', 'faq'],
  );
});

test('creates Vue-aligned defaults without sharing mutable nested values', () => {
  const first = defaultKnowledgeBaseEditorState('document');
  const second = defaultKnowledgeBaseEditorState('document');

  assert.equal(first.chunking.chunkSize, 512);
  assert.equal(first.chunking.chunkOverlap, 80);
  assert.equal(first.chunking.strategy, 'auto');
  assert.equal(first.chunking.enableParentChild, true);
  assert.equal(first.processing.questionGeneration.enabled, true);
  assert.equal(first.vectorStore.id, '');
  assert.equal(first.faq.indexMode, 'question_only');

  first.chunking.separators.push('custom');
  first.processing.languages.push('zh');
  assert.deepEqual(second.chunking.separators, ['\n\n', '\n', '。', '！', '？', ';', '；']);
  assert.deepEqual(second.processing.languages, []);
});

test('normalizes an API-shaped record while preserving explicit zero and false values', () => {
  const state = normalizeKnowledgeBaseEditorState({
    type: 'document',
    name: 'Existing',
    chunking_config: {
      chunk_size: 0,
      chunk_overlap: 0,
      enable_parent_child: false,
      strategy: '',
      token_limit: 0,
      languages: [],
      table_metadata_instructions: '',
      parser_engine_rules: [],
    },
    indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true, graph_enabled: false },
    question_generation_config: { enabled: false, question_count: 0 },
    auto_tag_config: { enabled: false, max_tags: 0, skip_if_tagged: false },
    vector_store_source: 'tenant',
    vector_store_name: 'pgvector',
    vector_store_engine_type: 'pgvector',
    vector_store_status: 'ready',
  });

  assert.equal(state.name, 'Existing');
  assert.equal(state.chunking.chunkSize, 0);
  assert.equal(state.chunking.chunkOverlap, 0);
  assert.equal(state.chunking.enableParentChild, false);
  assert.equal(state.indexing.vectorEnabled, false);
  assert.equal(state.processing.questionGeneration.enabled, false);
  assert.equal(state.processing.questionGeneration.questionCount, 0);
  assert.equal(state.processing.autoTag.skipIfTagged, false);
  assert.deepEqual(state.vectorStore, {
    id: '', source: 'tenant', name: 'pgvector', engineType: 'pgvector', status: 'ready',
  });
});

test('validates required name, at least one document index, and model dependencies', () => {
  const state = defaultKnowledgeBaseEditorState('document');
  state.indexing = { vectorEnabled: false, keywordEnabled: false, wikiEnabled: false, graphEnabled: false };
  state.model.embeddingModelId = '';
  state.model.llmModelId = '';

  assert.deepEqual(validateKnowledgeBaseEditorState(state), [
    { field: 'name', section: 'basic', message: 'Knowledge-base name is required' },
    { field: 'indexing', section: 'basic', message: 'At least one indexing strategy is required' },
    { field: 'summary_model_id', section: 'models', message: 'Summary model is required' },
  ]);

  state.name = 'Docs';
  state.indexing.vectorEnabled = true;
  assert.deepEqual(validateKnowledgeBaseEditorState(state), [
    { field: 'embedding_model_id', section: 'models', message: 'Embedding model is required' },
    { field: 'summary_model_id', section: 'models', message: 'Summary model is required' },
  ]);
});

test('does not require embedding for FAQ or wiki/graph-only documents', () => {
  const faq = defaultKnowledgeBaseEditorState('faq');
  faq.name = 'FAQ';
  faq.model.llmModelId = 'llm-1';
  assert.deepEqual(validateKnowledgeBaseEditorState(faq), []);

  const wiki = defaultKnowledgeBaseEditorState('document');
  wiki.name = 'Wiki';
  wiki.indexing = { vectorEnabled: false, keywordEnabled: false, wikiEnabled: true, graphEnabled: false };
  wiki.model.llmModelId = 'llm-1';
  assert.deepEqual(validateKnowledgeBaseEditorState(wiki), []);
});

test('builds create payload with FAQ, parser, processing, and selected vector store fields', () => {
  const state = defaultKnowledgeBaseEditorState('faq');
  state.name = 'Support FAQ';
  state.description = 'Answers';
  state.model.llmModelId = 'llm-1';
  state.vectorStore.id = 'store-1';
  state.faq = { indexMode: 'question_answer', questionIndexMode: 'combined' };
  state.processing.parserEngineRules = [{ parser: 'mineru', fileTypes: ['pdf'] }];
  state.processing.chunkSize = 0;
  state.processing.chunkOverlap = 0;
  state.processing.strategy = '';
  state.processing.tokenLimit = 0;

  assert.deepEqual(buildKnowledgeBasePayload(state, 'create'), {
    name: 'Support FAQ',
    description: 'Answers',
    type: 'faq',
    chunking_config: {
      chunk_size: 0,
      chunk_overlap: 0,
      separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
      enable_parent_child: true,
      parent_chunk_size: 4096,
      child_chunk_size: 384,
      strategy: '',
      token_limit: 0,
      languages: [],
      table_metadata_instructions: '',
      parser_engine_rules: [{ parser: 'mineru', fileTypes: ['pdf'] }],
    },
    embedding_model_id: '',
    summary_model_id: 'llm-1',
    vector_store_id: 'store-1',
    vlm_config: { enabled: false, model_id: '', description_language: '', custom_instructions: '' },
    asr_config: { enabled: false, model_id: '', language: '' },
    storage_provider_config: { provider: 'local' },
    storage_config: { provider: 'local' },
    question_generation_config: { enabled: true, question_count: 3, custom_instructions: '' },
    auto_tag_config: { enabled: false, model_id: '', max_tags: 3, skip_if_tagged: true },
    faq_config: { index_mode: 'question_answer', question_index_mode: 'combined' },
    extract_config: { enabled: false, text: '', tags: [], nodes: [], relations: [], custom_instructions: '' },
  });
});

test('omits vector-store binding on empty create and every update payload', () => {
  const state = defaultKnowledgeBaseEditorState('document');
  state.name = 'Docs';
  state.model.llmModelId = 'llm-1';
  state.model.embeddingModelId = 'embed-1';

  const create = buildKnowledgeBasePayload(state, 'create');
  const update = buildKnowledgeBasePayload(state, 'update');
  assert.equal('vector_store_id' in create, false);
  assert.equal('vector_store_id' in update, false);
  assert.equal('faq_config' in create, false);
  assert.equal('indexing_strategy' in (update.config ?? {}), true);
  assert.equal('parser_engine_rules' in update.config.chunking_config, false);
});

test('update payload separates base/config updates and keeps explicit processing clears', () => {
  const state = defaultKnowledgeBaseEditorState('document');
  state.name = 'Docs';
  state.description = 'Updated';
  state.model.llmModelId = 'llm-1';
  state.model.embeddingModelId = 'embed-1';
  state.processing.strategy = '';
  state.processing.tokenLimit = 0;
  state.processing.languages = [];
  state.processing.tableMetadataInstructions = '';
  state.processing.multimodal.enabled = true;
  state.processing.multimodal.vllmModelId = 'vlm-1';

  const payload = buildKnowledgeBasePayload(state, 'update');
  assert.deepEqual(payload.base, { name: 'Docs', description: 'Updated', config: {
    wiki_config: {
      synthesis_model_id: '', max_pages_per_ingest: 0, extraction_granularity: 'standard',
      content_instructions: '', extraction_instructions: '',
    },
    auto_tag_config: { enabled: false, model_id: '', max_tags: 3, skip_if_tagged: true },
    indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: false },
  } });
  assert.equal(payload.config.chunking_config.strategy, '');
  assert.equal(payload.config.chunking_config.token_limit, 0);
  assert.deepEqual(payload.config.chunking_config.languages, []);
  assert.deepEqual(payload.config.vlm_config, {
    enabled: true, model_id: 'vlm-1', description_language: '', custom_instructions: '',
  });
});

void ({} as KnowledgeBaseEditorState);
