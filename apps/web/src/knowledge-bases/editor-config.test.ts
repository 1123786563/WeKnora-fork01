import assert from 'node:assert/strict';
import test from 'node:test';
import { defaultKnowledgeEditorConfig, hydrateKnowledgeEditorConfig, knowledgeEditorConfigPayload } from './editor-config.ts';

test('hydrates Vue editor config and preserves nested defaults', () => {
  const config = hydrateKnowledgeEditorConfig({ indexing_strategy: { graph_enabled: true }, chunking_config: { chunk_size: 1024 }, faq_config: { index_mode: 'question_answer' } });
  assert.equal(config.chunkingConfig.chunkSize, 1024);
  assert.equal(config.faqConfig.indexMode, 'question_answer');
  assert.equal(config.chunkingConfig.enableParentChild, false);
});

test('uses Vue edit-mode fallbacks instead of create-mode defaults', () => {
  const config = hydrateKnowledgeEditorConfig({});
  assert.equal(config.chunkingConfig.strategy, '');
  assert.equal(config.chunkingConfig.enableParentChild, false);
  assert.equal(config.questionGenerationConfig.enabled, false);
});

test('builds document and FAQ payload boundaries like Vue', () => {
  const config = defaultKnowledgeEditorConfig();
  config.vectorStoreId = 'vs-1';
  config.nodeExtractConfig.enabled = true;
  assert.equal(knowledgeEditorConfigPayload(config, 'document').vector_store_id, 'vs-1');
  assert.equal((knowledgeEditorConfigPayload(config, 'faq') as Record<string, unknown>).indexing_strategy, undefined);
  assert.deepEqual((knowledgeEditorConfigPayload(config, 'faq').faq_config as Record<string, unknown>).index_mode, 'question_only');
});

test('uses real newline separator values like the Vue create defaults', () => {
  const config = defaultKnowledgeEditorConfig();
  assert.deepEqual(config.chunkingConfig.separators.slice(0, 2), ['\n\n', '\n']);
  const payload = knowledgeEditorConfigPayload(config, 'document');
  assert.deepEqual((payload.chunking_config as Record<string, unknown>).separators, config.chunkingConfig.separators);
});

test('uses Vue fallback defaults when numeric controls are cleared or set to zero', () => {
  const config = defaultKnowledgeEditorConfig();
  config.questionGenerationConfig.questionCount = 0;
  config.autoTagConfig.maxTags = 0;
  const payload = knowledgeEditorConfigPayload(config, 'document');
  assert.equal((payload.question_generation_config as Record<string, unknown>).question_count, 3);
  assert.equal((payload.auto_tag_config as Record<string, unknown>).max_tags, 3);
  assert.equal((payload.wiki_config as Record<string, unknown>).max_pages_per_ingest, 0);
});

test('round-trips the Vue storage provider compatibility fields', () => {
  const config = hydrateKnowledgeEditorConfig({
    storage_provider_config: { provider: 's3' },
    storage_config: { provider: 'local' },
  });
  assert.equal(config.storageProvider, 's3');

  const payload = knowledgeEditorConfigPayload(config, 'document');
  assert.deepEqual(payload.storage_provider_config, { provider: 's3' });
  assert.deepEqual(payload.storage_config, { provider: 's3' });
});

test('keeps disabled question generation like the Vue submit payload', () => {
  const config = defaultKnowledgeEditorConfig();
  config.questionGenerationConfig.enabled = false;
  const payload = knowledgeEditorConfigPayload(config, 'document');
  assert.deepEqual(payload.question_generation_config, {
    enabled: false,
    question_count: 3,
    custom_instructions: '',
  });
});

test('round-trips the Vue indexing strategy instead of hard-coding document defaults', () => {
  const config = hydrateKnowledgeEditorConfig({
    indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true, graph_enabled: true },
  });
  const payload = knowledgeEditorConfigPayload(config, 'document');
  assert.deepEqual(payload.indexing_strategy, {
    vector_enabled: false,
    keyword_enabled: false,
    wiki_enabled: true,
    graph_enabled: true,
  });
});

test('uses Vue fallback semantics for zero-valued chunk controls', () => {
  const config = hydrateKnowledgeEditorConfig({
    chunking_config: { chunk_size: 0, chunk_overlap: 0, parent_chunk_size: 0, child_chunk_size: 0, token_limit: 0 },
  });
  assert.equal(config.chunkingConfig.chunkSize, 512);
  assert.equal(config.chunkingConfig.chunkOverlap, 80);
  assert.equal(config.chunkingConfig.parentChunkSize, 4096);
  assert.equal(config.chunkingConfig.childChunkSize, 384);
  assert.equal(config.chunkingConfig.tokenLimit, 0);
});
