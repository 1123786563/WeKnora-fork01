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
