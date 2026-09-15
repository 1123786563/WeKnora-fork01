import assert from 'node:assert/strict';
import test from 'node:test';

import { buildKnowledgeBaseSettingsInput, formFromKnowledgeBase, updateParserRule, type KnowledgeBaseSettingsForm } from './form.ts';
import { formatMessage } from '@weknora/i18n';

test('knowledge-settings mutation fallback copy is localized in supported locales', () => {
  for (const locale of ['zh-CN', 'en-US'] as const) {
    assert.notEqual(formatMessage(locale, 'knowledgeEditor.messages.nameRequired'), 'knowledgeEditor.messages.nameRequired');
    assert.notEqual(formatMessage(locale, 'knowledgeEditor.messages.loadDataFailed'), 'knowledgeEditor.messages.loadDataFailed');
    assert.notEqual(formatMessage(locale, 'knowledgeEditor.messages.updateSuccess'), 'knowledgeEditor.messages.updateSuccess');
    assert.notEqual(formatMessage(locale, 'knowledgeEditor.chunking.debug.errorPrefix'), 'knowledgeEditor.chunking.debug.errorPrefix');
  }
});

test('maps a KB response into explicit settings fields and preserves section payloads', () => {
  const form = formFromKnowledgeBase({
    id: 'kb-1', name: 'Docs', description: 'Team docs',
    chunking_config: { chunk_size: 768, chunk_overlap: 48, strategy: 'auto', parser_engine_rules: [{ file_types: ['pdf'], engine: 'builtin' }] },
    embedding_model_id: 'embed-1', summary_model_id: 'summary-1', storage_backend_id: 'storage-1', vector_store_id: 'vector-1',
    indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: true },
  });
  assert.equal(form.chunkSize, 768);
  assert.equal(form.chunkOverlap, 48);
  assert.equal(form.embeddingModelId, 'embed-1');
  assert.equal(form.storageBackendId, 'storage-1');
  assert.equal(form.parserRules[0]?.engine, 'builtin');
  const input = buildKnowledgeBaseSettingsInput(form);
  assert.equal(input.name, 'Docs');
  assert.deepEqual(input.config?.chunking_config, { chunk_size: 768, chunk_overlap: 48, strategy: 'auto', enable_parent_child: false, parent_chunk_size: 4096, child_chunk_size: 384, token_limit: 0, parser_engine_rules: [{ file_types: ['pdf'], engine: 'builtin' }], table_metadata_instructions: '' });
  assert.deepEqual(input.config?.indexing_strategy, { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: true });
  assert.equal((input.config as Record<string, unknown>).vector_store_id, undefined);
});

test('updates one parser group without switching to a generic JSON editor', () => {
  const form: KnowledgeBaseSettingsForm = formFromKnowledgeBase({ id: 'kb-1', name: 'Docs' });
  const next = updateParserRule(form.parserRules, ['pdf', 'docx'], 'anydoc');
  assert.deepEqual(next, [{ file_types: ['pdf', 'docx'], engine: 'anydoc' }]);
  assert.deepEqual(updateParserRule(next, ['pdf'], ''), [{ file_types: ['docx'], engine: 'anydoc' }]);
});
