import assert from 'node:assert/strict';
import test from 'node:test';

import { createKnowledgeSettingsApi } from './settings.ts';

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
      return { selected_tier: 'recursive', tier_chain: ['recursive'], rejected: [], chunks: [], stats: { count: 0, avg_chars: 0, min_chars: 0, max_chars: 0, stddev_chars: 0 } };
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
