import assert from 'node:assert/strict';
import test from 'node:test';

import { createKnowledgeFaqApi } from './faq.ts';

const entry = { id: 7, standard_question: 'How?', similar_questions: [], negative_questions: [], answers: ['This.'], is_enabled: true, is_recommended: false };

test('parses the nested FAQ list envelope and preserves explicit enabled filtering', async () => {
  let path = '';
  const api = createKnowledgeFaqApi(async (request) => { path = request.path; return { success: true, data: { total: 1, page: 1, page_size: 20, data: [entry] } }; });
  const result = await api.list('kb/a', { keyword: 'how', is_enabled: false });
  assert.equal(result.data[0]?.id, 7);
  assert.equal(path, '/api/v1/knowledge-bases/kb%2Fa/faq/entries?keyword=how&is_enabled=false');
});

test('normalizes nullable FAQ negative questions from the backend', async () => {
  const api = createKnowledgeFaqApi(async () => ({
    success: true,
    data: { total: 1, page: 1, page_size: 20, data: [{ ...entry, similar_questions: null, negative_questions: null }] },
  }));

  const result = await api.list('kb-1');

  assert.deepEqual(result.data[0]?.similar_questions, []);
  assert.deepEqual(result.data[0]?.negative_questions, []);
});

test('keeps FAQ writes typed and reports import task identity', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createKnowledgeFaqApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    if (request.method === 'POST' && request.path.endsWith('/entries')) return { success: true, data: { task_id: 'task-1' } };
    if (request.method === 'DELETE') return undefined;
    return { success: true, data: entry };
  });
  assert.equal((await api.create('kb-1', entry)).id, 7);
  assert.equal((await api.update('kb-1', 7, { answers: ['Updated'] })).id, 7);
  assert.equal((await api.upsert('kb-1', { entries: [entry], mode: 'append' })).task_id, 'task-1');
  await api.updateFields('kb-1', { by_id: { 7: { is_enabled: false } } });
  await api.updateTags('kb-1', { updates: { 7: 2 } });
  await api.removeMany('kb-1', [7]);
  assert.deepEqual(requests.map(({ method, path }) => `${method} ${path}`), [
    'POST /api/v1/knowledge-bases/kb-1/faq/entry',
    'PUT /api/v1/knowledge-bases/kb-1/faq/entries/7',
    'POST /api/v1/knowledge-bases/kb-1/faq/entries',
    'PUT /api/v1/knowledge-bases/kb-1/faq/entries/fields',
    'PUT /api/v1/knowledge-bases/kb-1/faq/entries/tags',
    'DELETE /api/v1/knowledge-bases/kb-1/faq/entries',
  ]);
});

test('uses the server search and export endpoints without changing their payloads', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createKnowledgeFaqApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    return request.path.includes('/export') ? 'standard_question,answers\n"How?","This."\n' : { success: true, data: { results: [] } };
  });
  await api.search('kb-1', { query_text: 'How?', vector_threshold: 0.7, match_count: 5 });
  assert.equal(await api.exportEntries('kb-1'), 'standard_question,answers\n"How?","This."\n');
  assert.equal(await api.exportEntries('kb-1', 'json'), 'standard_question,answers\n"How?","This."\n');
  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/knowledge-bases/kb-1/faq/search', body: { query_text: 'How?', vector_threshold: 0.7, match_count: 5 } },
    { method: 'GET', path: '/api/v1/knowledge-bases/kb-1/faq/entries/export', body: undefined },
    { method: 'GET', path: '/api/v1/knowledge-bases/kb-1/faq/entries/export?format=json', body: undefined },
  ]);
});

test('importProgress fetches the backend task progress envelope', async () => {
  let path = '';
  const api = createKnowledgeFaqApi(async (request) => {
    path = request.path;
    return { success: true, data: { task_id: 'task-9', kb_id: 'kb-1', knowledge_id: 'k-1', status: 'processing', progress: 40, total: 5, processed: 2 } };
  });
  const progress = await api.importProgress('task-9');
  assert.equal(path, '/api/v1/faq/import/progress/task-9');
  assert.equal(progress.status, 'processing');
  assert.equal(progress.processed, 2);
  assert.equal(progress.total, 5);
});

test('importProgress rejects an empty task id', async () => {
  const api = createKnowledgeFaqApi(async () => ({ success: true, data: {} }));
  await assert.rejects(() => api.importProgress(''), /Invalid FAQ task id/);
});
