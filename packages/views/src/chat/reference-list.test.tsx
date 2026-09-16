import assert from 'node:assert/strict';
import test from 'node:test';

import { referenceSections } from './reference-list.tsx';

test('groups references with safe web links and activatable local references', () => {
  const groups = referenceSections([
    { id: 'chunk-1', knowledge_id: 'doc-1', knowledge_title: 'Refund policy', content: 'Five days.' },
    { id: 'https://example.com/guide', chunk_type: 'web_search', metadata: { url: 'https://example.com/guide', title: 'Guide' }, content: 'Web result.' },
    { id: 'tool-1', chunk_type: 'tool_result', knowledge_title: 'Query', content: '2 rows' },
  ]);

  assert.deepEqual(groups.map((group) => group.id), ['web', 'document', 'tool']);
  assert.equal(groups[0]?.items[0]?.url, 'https://example.com/guide');
  assert.deepEqual(groups[1]?.items[0]?.chunkIds, ['chunk-1']);
  assert.match(groups[1]?.items[0]?.snippet ?? '', /days\./);
  assert.equal(groups[2]?.items[0]?.chunkId, 'tool-1');
});

test('does not create a panel model when all reference entries are invalid', () => {
  assert.deepEqual(referenceSections([{ id: 'web-1', chunk_type: 'web_search', metadata: { url: 'javascript:alert(1)' } }]), []);
});
