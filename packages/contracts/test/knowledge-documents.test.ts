import assert from 'node:assert/strict';
import test from 'node:test';

import { ContractError, parseKnowledgeDocumentListResponse, parseKnowledgeFolderTreeResponse, parseKnowledgeSearchResponse, parseKnowledgeTagListResponse } from '../src/index.ts';

test('parses the paginated knowledge document envelope', () => {
  const result = parseKnowledgeDocumentListResponse({
    success: true,
    data: [{ id: 'doc-1', parse_status: 'processing', file_name: 'a.pdf' }],
    total: 1,
    page: 1,
    page_size: 20,
  });
  assert.equal(result.data[0]?.id, 'doc-1');
  assert.equal(result.page_size, 20);
});

test('rejects malformed pagination instead of guessing', () => {
  assert.throws(
    () => parseKnowledgeDocumentListResponse({ success: true, data: [], total: '1', page: 1, page_size: 20 }),
    (error: unknown) => error instanceof ContractError && error.path === 'total',
  );
});

test('parses folder tree and tags without weakening numeric or identity fields', () => {
  const folders = parseKnowledgeFolderTreeResponse({
    success: true,
    data: { root_document_count: 1, total_document_count: 2, folders: [{ path: 'docs', name: 'docs', document_count: 1, total_count: 1, children: [] }] },
  });
  const tags = parseKnowledgeTagListResponse({ success: true, data: [{ id: 'tag-1', seq_id: 4, name: 'important' }] });
  assert.equal(folders.folders[0]?.children?.length, 0);
  assert.equal(tags.data[0]?.seq_id, 4);
  assert.throws(() => parseKnowledgeTagListResponse({ success: true, data: [{ id: 'tag-1', name: 4 }] }), ContractError);
});

test('parses the backend paginated tag envelope', () => {
  const result = parseKnowledgeTagListResponse({
    success: true,
    data: { total: 1, page: 1, page_size: 100, data: [{ id: 'tag-1', name: 'important' }] },
  });
  assert.equal(result.data[0]?.name, 'important');
  assert.equal(result.total, 1);
  assert.equal(result.page_size, 100);
});

test('parses recent search response and rejects missing completion metadata', () => {
  const result = parseKnowledgeSearchResponse({ success: true, data: [{ id: 'doc-1', title: 'Guide' }], has_more: true, total: 3 });
  assert.equal(result.has_more, true);
  assert.throws(() => parseKnowledgeSearchResponse({ success: true, data: [], total: 0 }), ContractError);
});
