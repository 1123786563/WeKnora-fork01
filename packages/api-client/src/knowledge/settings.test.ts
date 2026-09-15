import assert from 'node:assert/strict';
import test from 'node:test';

import {
  parseKnowledgeBaseConfigInput,
  parseKnowledgeBaseConfigUpdateInput,
  type KnowledgeBaseConfigInput,
} from './settings.ts';

const createPayload: KnowledgeBaseConfigInput = {
  name: 'Docs',
  description: '',
  type: 'document',
  chunking_config: {
    chunk_size: 512,
    chunk_overlap: 80,
    separators: ['\n\n', '\n'],
    enable_parent_child: true,
    parent_chunk_size: 4096,
    child_chunk_size: 384,
    strategy: '',
    token_limit: 0,
    languages: [],
    table_metadata_instructions: '',
    parser_engine_rules: [{ parser: 'mineru', fileTypes: ['pdf'] }],
  },
  embedding_model_id: 'embed-1',
  summary_model_id: 'llm-1',
  vlm_config: { enabled: false, model_id: '', description_language: '', custom_instructions: '' },
  asr_config: { enabled: false, model_id: '', language: '' },
  storage_provider_config: { provider: 'local' },
  storage_config: { provider: 'local' },
  question_generation_config: { enabled: false, question_count: 3, custom_instructions: '' },
  auto_tag_config: { enabled: false, model_id: '', max_tags: 3, skip_if_tagged: true },
  indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: false },
  extract_config: { enabled: false, text: '', tags: [], nodes: [], relations: [], custom_instructions: '' },
};

test('parses the Vue editor create payload and preserves explicit empty values', () => {
  const parsed = parseKnowledgeBaseConfigInput(createPayload);

  assert.deepEqual(parsed, createPayload);
  assert.equal(parsed.chunking_config?.token_limit, 0);
  assert.deepEqual(parsed.chunking_config?.languages, []);
  assert.equal(parsed.vlm_config?.enabled, false);
});

test('parses update payloads without dropping nested config clears or legacy fields', () => {
  const payload = {
    name: 'Updated',
    description: '',
    config: {
      indexing_strategy: { vector_enabled: false, keyword_enabled: true, wiki_enabled: false, graph_enabled: false },
      auto_tag_config: { enabled: false, model_id: '', max_tags: 0, skip_if_tagged: false },
      wiki_config: {
        synthesis_model_id: '',
        max_pages_per_ingest: 0,
        extraction_granularity: 'standard',
        content_instructions: '',
        extraction_instructions: '',
      },
      legacy_option: { enabled: true },
    },
    legacy_top_level: true,
  };

  assert.deepEqual(parseKnowledgeBaseConfigUpdateInput(payload), payload);
});

test('rejects malformed scalar, nested, and parser-rule values', () => {
  assert.throws(() => parseKnowledgeBaseConfigInput({ ...createPayload, name: 42 }), /name/);
  assert.throws(
    () => parseKnowledgeBaseConfigInput({ ...createPayload, chunking_config: { ...createPayload.chunking_config, token_limit: '0' } }),
    /token_limit/,
  );
  assert.throws(
    () => parseKnowledgeBaseConfigInput({
      ...createPayload,
      chunking_config: { ...createPayload.chunking_config, parser_engine_rules: [{ parser: 'mineru', fileTypes: [1] }] },
    }),
    /fileTypes/,
  );
  assert.throws(() => parseKnowledgeBaseConfigUpdateInput({ config: null }), /config/);
});

test('rejects arrays and non-plain objects instead of accepting arbitrary JSON values', () => {
  assert.throws(() => parseKnowledgeBaseConfigInput([]), /object/);
  assert.throws(() => parseKnowledgeBaseConfigInput({ ...createPayload, chunking_config: [] }), /chunking_config/);
  assert.throws(() => parseKnowledgeBaseConfigUpdateInput({ __proto__: { polluted: true } }), /object/);
});
