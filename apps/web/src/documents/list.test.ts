import assert from 'node:assert/strict';
import test from 'node:test';
import type { KnowledgeDocumentListParams } from '@weknora/api-client';

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

test('normalizes root-folder filters to the Vue folder scope before loading', async () => {
  let received: unknown;
  await loadKnowledgeDocuments({
    knowledgeBases: {
      documents: {
        list: async (_knowledgeBaseId: string, params?: KnowledgeDocumentListParams) => {
          received = params;
          return { data: [], total: 0, page: 1, page_size: 20 };
        },
      },
    },
  } as never, 'kb-1', {
    page: 1,
    page_size: 20,
    keyword: '  release notes  ',
    tag_ids: 'tag-1,tag-2',
    file_type: 'pdf',
    parse_status: 'failed',
    source: 'web',
    start_time: '2026-09-01 00:00:00',
    end_time: '2026-09-15 23:59:59',
    folder_path: undefined,
    folder_recursive: false,
  });

  assert.deepEqual(received, {
    page: 1,
    page_size: 20,
    keyword: 'release notes',
    tag_ids: 'tag-1,tag-2',
    file_type: 'pdf',
    parse_status: 'failed',
    source: 'web',
    start_time: '2026-09-01 00:00:00',
    end_time: '2026-09-15 23:59:59',
    folder_path: '',
    folder_recursive: true,
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
