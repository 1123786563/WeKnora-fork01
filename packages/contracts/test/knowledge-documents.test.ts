import assert from 'node:assert/strict';
import test from 'node:test';

import { ContractError, parseKnowledgeDocumentListResponse } from '../src/index.ts';

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
