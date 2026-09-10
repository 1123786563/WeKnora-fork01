import assert from 'node:assert/strict';
import test from 'node:test';

import { loadKnowledgeBases } from './list.ts';

test('loads real client data into a renderable list state', async () => {
  const state = await loadKnowledgeBases({
    knowledgeBases: {
      list: async () => [{ id: 'kb-1', name: 'Docs' }],
    },
  } as never);

  assert.deepEqual(state, { status: 'success', items: [{ id: 'kb-1', name: 'Docs' }] });
});
test('converts client failures into an error state without fallback data', async () => {
  const state = await loadKnowledgeBases({
    knowledgeBases: {
      list: async () => { throw new Error('backend unavailable'); },
    },
  } as never);

  assert.deepEqual(state, { status: 'error', message: 'backend unavailable' });
});
