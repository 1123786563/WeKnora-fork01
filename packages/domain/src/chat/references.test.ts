import assert from 'node:assert/strict';
import test from 'node:test';

import { groupChatReferences } from './references.ts';

test('groups duplicate document chunks and keeps web and tool references separate', () => {
  const groups = groupChatReferences([
    {
      id: 'chunk-1',
      knowledge_id: 'document-1',
      knowledge_title: 'Refund policy',
      knowledge_base_id: 'kb-1',
      content: 'Refunds take five days.',
    },
    {
      id: 'chunk-2',
      knowledge_id: 'document-1',
      knowledge_title: 'Refund policy',
      knowledge_base_id: 'kb-1',
      content: 'Card refunds can take longer.',
    },
    {
      id: 'https://www.example.com/news/',
      chunk_type: 'web_search',
      metadata: { url: 'https://www.example.com/news/#latest', title: 'Example news' },
      content: 'A web result.',
    },
    {
      id: 'tool-1',
      chunk_type: 'tool_result',
      knowledge_title: 'Database query',
      metadata: { tool: 'query_database' },
      content: '2 rows',
    },
  ]);

  assert.deepEqual(groups.map((group) => group.kind), ['web', 'document', 'tool']);
  assert.equal(groups[0]?.items[0]?.url, 'https://www.example.com/news');
  assert.equal(groups[1]?.items[0]?.key, 'document:document-1');
  assert.deepEqual(groups[1]?.items[0]?.chunkIds, ['chunk-1', 'chunk-2']);
  assert.match(groups[1]?.items[0]?.content ?? '', /Refunds take five days/);
  assert.match(groups[1]?.items[0]?.content ?? '', /Card refunds can take longer/);
  assert.equal(groups[2]?.items[0]?.source, 'query_database');
});

test('drops unsafe web destinations instead of turning them into clickable references', () => {
  const groups = groupChatReferences([
    {
      id: 'web-1',
      chunk_type: 'web_search',
      metadata: { url: 'javascript:alert(1)', title: 'Unsafe' },
      content: 'Do not run this.',
    },
    null,
    'not a reference',
  ]);

  assert.deepEqual(groups, []);
});
