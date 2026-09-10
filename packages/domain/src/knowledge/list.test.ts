import assert from 'node:assert/strict';
import test from 'node:test';

import { filterKnowledgeBases } from './list.ts';

const items = [
  { id: 'kb-1', name: 'Product docs', type: 'document', creator_id: 'user-1', is_favorite: true },
  { id: 'kb-2', name: 'Support FAQ', type: 'faq', creator_id: 'user-2', source: 'organization', permission: 'viewer' },
  { id: 'kb-3', name: 'Private notes', type: 'document', creator_id: 'user-1' },
];

test('filters knowledge bases by query, type, creator, and preserves source metadata', () => {
  const result = filterKnowledgeBases(items, { query: 'faq', type: 'faq', creator: 'others', currentUserId: 'user-1' });
  assert.deepEqual(result, { items: [items[1]], total: 1, page: 1, pageCount: 1 });
  assert.equal((result.items[0] as Record<string, unknown>).source, 'organization');
});

test('paginates filtered knowledge bases and clamps an out-of-range page', () => {
  const result = filterKnowledgeBases(items, { page: 3, pageSize: 2 });
  assert.deepEqual(result, { items: [items[2]], total: 3, page: 2, pageCount: 2 });
});

test('supports a favorites-only view without treating missing favorite state as true', () => {
  const result = filterKnowledgeBases(items, { favoritesOnly: true });
  assert.deepEqual(result.items, [items[0]]);
});
