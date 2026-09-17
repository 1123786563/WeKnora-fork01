import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createKnowledgeSettingsApi,
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

test('loads and updates KB settings through the concrete endpoints', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createKnowledgeSettingsApi(async (input) => {
    requests.push(input);
    if (input.method === 'GET' && input.path.includes('/activity')) {
      return { success: true, data: [{ id: 7, action: 'kb.updated', outcome: 'success', created_at: '2026-09-11T00:00:00Z' }], next_cursor: 0 };
    }
    if (input.method === 'GET' && input.path === '/api/v1/system/parser-engines') {
      return { code: 0, data: [{ Name: 'builtin', Description: 'Built in', FileTypes: ['txt'] }], connected: true };
    }
    if (input.method === 'POST' && input.path === '/api/v1/chunker/preview') {
      // The Go handler wraps PreviewChunkingResponse in a {success, data} envelope.
      return { success: true, data: { selected_tier: 'recursive', tier_chain: ['recursive'], rejected: [], chunks: [], stats: { count: 0, avg_chars: 0, min_chars: 0, max_chars: 0, stddev_chars: 0 } } };
    }
    if (input.method === 'POST' && input.path === '/api/v1/chunker/preview-null-rejected') {
      // Live backend shape (captured R448, Parity KB Demo): `rejected` arrives
      // as null when nothing was rejected; profile carries nested objects.
      return { success: true, data: { selected_tier: 'legacy', tier_chain: ['legacy'], rejected: null, profile: { total_chars: 36, has_tables: false, detected_langs: ['zh'] }, chunks: [{ seq: 0, start: 0, end: 36, size_chars: 36, content: 'x' }], stats: { count: 1, avg_chars: 36, min_chars: 36, max_chars: 36, stddev_chars: 0 } } };
    }
    if (input.method === 'GET' && input.path === '/api/v1/storage-backends') {
      return { success: true, data: [{ id: 'storage-1', name: 'Local', provider: 'local', config: {}, source: 'env', status: 'active' }] };
    }
    if (input.method === 'GET' && input.path === '/api/v1/vector-stores') {
      return { success: true, data: [{ id: 'vector-1', name: 'Vector', engine_type: 'pgvector', connection_config: {}, index_config: {}, source: 'user', readonly: false }] };
    }
    return { success: true, data: { id: 'kb-1', name: 'Docs', type: 'document', indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: false } } };
  });

  assert.equal((await api.get('kb/a')).id, 'kb-1');
  assert.equal((await api.update('kb/a', { name: 'Docs', config: {} })).name, 'Docs');
  assert.equal((await api.parserEngines()).data[0]?.Name, 'builtin');
  assert.equal((await api.previewChunking({ text: 'hello', chunking_config: { chunk_size: 128 } })).selected_tier, 'recursive');
  assert.equal((await api.storageBackends()).data[0]?.id, 'storage-1');
  assert.equal((await api.vectorStores()).data[0]?.id, 'vector-1');
  assert.equal((await api.activity('kb/a')).data?.[0]?.id, 7);
  assert.equal(requests[1]?.path, '/api/v1/knowledge-bases/kb%2Fa');
  assert.deepEqual(requests[2]?.path, '/api/v1/system/parser-engines');
});

test('rejects malformed parser and resource responses instead of fabricating settings', async () => {
  const api = createKnowledgeSettingsApi(async () => ({ code: 0, data: [{ Name: 'broken' }] }));
  await assert.rejects(api.parserEngines(), /Invalid parser engine/);
});

test('rejects unsafe activity numbers and malformed optional parser fields', async () => {
  const activity = createKnowledgeSettingsApi(async () => ({ success: true, data: [{ id: 1.5, action: 'kb.updated', outcome: 'success', created_at: '2026-09-11T00:00:00Z' }] }));
  await assert.rejects(activity.activity('kb-1'), /Invalid knowledge base activity/);

  const parser = createKnowledgeSettingsApi(async () => ({ code: 0, data: [{ Name: 'builtin', Description: 'Built in', FileTypes: ['txt'], Available: 'yes' }] }));
  await assert.rejects(parser.parserEngines(), /Invalid parser engine/);
});

test('rejects malformed chunking stats and default storage identifiers', async () => {
  const chunking = createKnowledgeSettingsApi(async () => ({ success: true, data: { selected_tier: 'recursive', tier_chain: ['recursive'], rejected: [], chunks: [], stats: { count: Number.NaN } } }));
  await assert.rejects(chunking.previewChunking({ text: 'hello', chunking_config: {} }), /Invalid chunking preview stats/);

  const storage = createKnowledgeSettingsApi(async () => ({ success: true, data: [], default_storage_backend_id: 42 }));
  await assert.rejects(storage.storageBackends(), /Invalid default storage backend/);
});

test('normalizes the live null rejected field captured from the real chunker preview', async () => {
  const api = createKnowledgeSettingsApi(async () => ({ success: true, data: { selected_tier: 'legacy', tier_chain: ['legacy'], rejected: null, profile: { total_chars: 36, has_tables: false, detected_langs: ['zh'] }, chunks: [{ seq: 0, start: 0, end: 36, size_chars: 36, content: 'x' }], stats: { count: 1, avg_chars: 36, min_chars: 36, max_chars: 36, stddev_chars: 0 } } }));
  const preview = await api.previewChunking({ text: 'hello', chunking_config: {} });
  assert.deepEqual(preview.rejected, [], 'the backend sends rejected:null when nothing was rejected — it must read as an empty list, not a parse failure');
  assert.equal(preview.selected_tier, 'legacy');
  assert.equal(preview.chunks.length, 1);
});
