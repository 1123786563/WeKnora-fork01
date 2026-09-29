import assert from 'node:assert/strict';
import test from 'node:test';
import type { KnowledgeDocumentListParams } from '@weknora/api-client';

import { documentListPageSize, loadKnowledgeDocuments, mergeDocumentListState } from './list.ts';

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

test('loads the Vue-authoritative success/data response shape', async () => {
  const result = await loadKnowledgeDocuments({
    knowledgeBases: {
      documents: {
        list: async () => ({
          success: true,
          data: [{ id: 'doc-1', parse_status: 'completed' }],
          total: 1,
          page: 2,
          page_size: 10,
        }),
      },
    },
  } as never, 'kb-1');

  assert.deepEqual(result, {
    status: 'success',
    page: { items: [{ id: 'doc-1', parse_status: 'completed' }], total: 1, page: 2, pageSize: 10 },
  });
});

test('does not render data from a failed Vue response envelope', async () => {
  const result = await loadKnowledgeDocuments({
    knowledgeBases: {
      documents: {
        list: async () => ({ success: false, data: [{ id: 'stale-doc' }], total: 1 }),
      },
    },
  } as never, 'kb-1');

  assert.deepEqual(result, { status: 'error', message: 'Invalid knowledge document page' });
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

test('documentListPageSize mirrors the Vue viewport-adaptive getPageSize (KnowledgeBase.vue L810-816)', () => {
  assert.equal(documentListPageSize(900), Math.max(35, Math.floor(900 / 148) * 5));
  assert.equal(documentListPageSize(768), 35);  // floor(768/148)=5 -> 25, floored at 35
  assert.equal(documentListPageSize(2000), Math.floor(2000 / 148) * 5);
});

test('mergeDocumentListState replaces on page 1 and appends on scroll pages', () => {
  const first = { status: 'success' as const, page: { items: [{ id: 'a' }, { id: 'b' }], total: 5, page: 1, pageSize: 2 } };
  const second = { status: 'success' as const, page: { items: [{ id: 'c' }, { id: 'd' }], total: 5, page: 2, pageSize: 2 } };

  assert.deepEqual(mergeDocumentListState({ status: 'loading' }, second, 1), second);
  assert.deepEqual(mergeDocumentListState(first, second, 1), second);
  assert.deepEqual(mergeDocumentListState(first, second, 2), {
    status: 'success',
    page: { items: [{ id: 'a' }, { id: 'b' }, { id: 'c' }, { id: 'd' }], total: 5, page: 2, pageSize: 2 },
  });
});

test('mergeDocumentListState dedupes re-delivered rows and keeps the list on a failed scroll load', () => {
  const first = { status: 'success' as const, page: { items: [{ id: 'a' }], total: 3, page: 1, pageSize: 1 } };
  const overlap = { status: 'success' as const, page: { items: [{ id: 'a' }, { id: 'b' }], total: 3, page: 2, pageSize: 1 } };
  assert.deepEqual(mergeDocumentListState(first, overlap, 2), {
    status: 'success',
    page: { items: [{ id: 'a' }, { id: 'b' }], total: 3, page: 2, pageSize: 1 },
  });

  const failed = { status: 'error' as const, message: 'boom' };
  assert.equal(mergeDocumentListState(first, failed, 2), first);
  assert.deepEqual(mergeDocumentListState({ status: 'loading' }, failed, 1), failed);
});
