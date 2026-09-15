import assert from 'node:assert/strict';
import test from 'node:test';
import { defaultKnowledgeEditorConfig, hydrateKnowledgeEditorConfig, knowledgeEditorConfigPayload } from './editor-config.ts';

test('hydrates Vue editor config and preserves nested defaults', () => {
  const config = hydrateKnowledgeEditorConfig({ indexing_strategy: { graph_enabled: true }, chunking_config: { chunk_size: 1024 }, faq_config: { index_mode: 'question_answer' } });
  assert.equal(config.chunkingConfig.chunkSize, 1024);
  assert.equal(config.faqConfig.indexMode, 'question_answer');
  assert.equal(config.chunkingConfig.enableParentChild, true);
});

test('builds document and FAQ payload boundaries like Vue', () => {
  const config = defaultKnowledgeEditorConfig();
  config.vectorStoreId = 'vs-1';
  config.nodeExtractConfig.enabled = true;
  assert.equal(knowledgeEditorConfigPayload(config, 'document').vector_store_id, 'vs-1');
  assert.equal((knowledgeEditorConfigPayload(config, 'faq') as Record<string, unknown>).indexing_strategy, undefined);
  assert.deepEqual((knowledgeEditorConfigPayload(config, 'faq').faq_config as Record<string, unknown>).index_mode, 'question_only');
});
