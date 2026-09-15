import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildKnowledgeDocumentSearchParams,
  loadKnowledgeBaseDetail,
  loadKnowledgeDocumentPreview,
} from './detail.ts';

test('loads the Vue knowledge-base detail endpoint and unwraps its envelope', async () => {
  let received: unknown;
  const result = await loadKnowledgeBaseDetail(async (input) => {
    received = input;
    return { success: true, data: { id: 'kb-1', name: 'Docs' } };
  }, 'kb/1');
  assert.deepEqual(received, { method: 'GET', path: '/api/v1/knowledge-bases/kb%2F1' });
  assert.deepEqual(result, { status: 'ready', knowledgeBase: { id: 'kb-1', name: 'Docs' } });
});

test('separates forbidden, empty, and ordinary preview failures', async () => {
  assert.deepEqual(await loadKnowledgeDocumentPreview(async () => ({ success: true, data: {} }), 'doc-1'), { status: 'empty' });
  assert.deepEqual(await loadKnowledgeDocumentPreview(async () => { throw Object.assign(new Error('denied'), { code: 'FORBIDDEN' }); }, 'doc-1'), {
    status: 'forbidden', code: 'FORBIDDEN', message: 'denied',
  });
  assert.deepEqual(await loadKnowledgeBaseDetail(async () => { throw new Error('offline'); }, 'kb-1'), {
    status: 'error', message: 'offline',
  });
});

test('builds Vue-compatible document search parameters and omits empty filters', () => {
  assert.deepEqual(buildKnowledgeDocumentSearchParams({
    keyword: '  handbook ', page: 2, pageSize: 20, folderPath: '', recursive: true, fileType: 'pdf', parseStatus: 'completed', source: '',
  }), {
    keyword: 'handbook', page: 2, page_size: 20, folder_path: '', folder_recursive: true, file_type: 'pdf', parse_status: 'completed',
  });
});
