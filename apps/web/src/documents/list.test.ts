import assert from 'node:assert/strict';
import test from 'node:test';

import { loadKnowledgeDocuments } from './list.ts';

test('loads documents through the api-client seam without fallback data', async () => {
  const result = await loadKnowledgeDocuments({
    knowledgeBases: {
      documents: {
        list: async () => ({ items: [{ id: 'doc-1', parse_status: 'pending' }], total: 1, page: 1, pageSize: 20 }),
      },
    },
  } as never, 'kb-1');

  assert.deepEqual(result, {
    status: 'success',
    page: { items: [{ id: 'doc-1', parse_status: 'pending' }], total: 1, page: 1, pageSize: 20 },
  });
});

test('uses the caller locale fallback when the document API is unavailable or returns an unknown error', async () => {
  const unavailable = await loadKnowledgeDocuments({}, 'kb-1', {}, {
    unavailable: '文档接口不可用',
    failed: '文档加载失败',
  });
  assert.deepEqual(unavailable, { status: 'error', message: '文档接口不可用' });

  const failed = await loadKnowledgeDocuments({
    knowledgeBases: { documents: { list: async () => { throw 'opaque'; } } },
  } as never, 'kb-1', {}, {
    unavailable: '文档接口不可用',
    failed: '文档加载失败',
  });
  assert.deepEqual(failed, { status: 'error', message: '文档加载失败' });
});
