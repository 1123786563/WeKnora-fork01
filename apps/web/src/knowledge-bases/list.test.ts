import assert from 'node:assert/strict';
import test from 'node:test';
import type { KnowledgeBaseListParams } from '@weknora/api-client';

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

test('preserves forbidden error codes for the no-permission branch', async () => {
  const state = await loadKnowledgeBases({
    knowledgeBases: { list: async () => { throw Object.assign(new Error('No access'), { code: 'TENANT_FORBIDDEN' }); } },
  } as never);

  assert.deepEqual(state, { status: 'error', code: 'TENANT_FORBIDDEN', message: 'No access' });
});

test('passes the Vue-equivalent creator scope to the backend list request', async () => {
  let received: unknown;
  const state = await loadKnowledgeBases({
    knowledgeBases: {
      list: async (params: KnowledgeBaseListParams = {}) => {
        received = params;
        return [];
      },
    },
  } as never, undefined, { creator: 'mine' });

  assert.deepEqual(received, { creator: 'mine' });
  assert.deepEqual(state, { status: 'success', items: [] });
});
