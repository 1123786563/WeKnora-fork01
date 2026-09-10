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

test('does not report a mutation success before the client resolves', async () => {
  const calls: string[] = [];
  const client = { knowledgeBases: {
    create: async (input: { name: string }) => { calls.push(`create:${input.name}`); return { id: 'kb-2', name: input.name }; },
    update: async (id: string, input: { name: string }) => { calls.push(`update:${id}:${input.name}`); return { id, name: input.name }; },
    remove: async (id: string) => { calls.push(`remove:${id}`); },
  } } as never;
  const { saveKnowledgeBase, deleteKnowledgeBase } = await import('./list.ts');
  assert.deepEqual(await saveKnowledgeBase(client, null, { name: 'FAQ', type: 'faq' }), { id: 'kb-2', name: 'FAQ' });
  await saveKnowledgeBase(client, 'kb-2', { name: 'FAQ v2' });
  await deleteKnowledgeBase(client, 'kb-2');
  assert.deepEqual(calls, ['create:FAQ', 'update:kb-2:FAQ v2', 'remove:kb-2']);
  await assert.rejects(saveKnowledgeBase(client, null, { name: ' ' }), /name is required/);
});

test('passes the creator filter to the server-compatible list query', async () => {
  let seen: unknown;
  await loadKnowledgeBases({ knowledgeBases: {
    list: async (params: unknown) => { seen = params; return []; },
  } } as never, undefined, { creator: 'mine' });
  assert.deepEqual(seen, { creator: 'mine' });
});
