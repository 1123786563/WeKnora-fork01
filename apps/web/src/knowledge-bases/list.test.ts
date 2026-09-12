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

// ---- Card-grid page loader and delete in-flight guard ----

import { createDeleteGuard, loadKnowledgeBaseListPage } from './list.ts';

test('loads owned and shared knowledge bases into a non-error loading model', async () => {
  const state = await loadKnowledgeBaseListPage({
    knowledgeBases: { list: async () => [{ id: 'kb-1', name: 'Docs' }] },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => [
      { knowledge_base: { id: 'kb-9' }, permission: 'viewer', shared_at: '2024-01-01T00:00:00Z', share_id: 's-1' },
    ] } } },
  } as never);
  assert.equal(state.status, 'success');
  if (state.status !== 'success') throw new Error('unreachable');
  assert.deepEqual(state.owned, [{ id: 'kb-1', name: 'Docs' }]);
  assert.equal(state.shared.length, 1);
});

test('a shared failure surfaces an error state even when owned succeeds', async () => {
  const state = await loadKnowledgeBaseListPage({
    knowledgeBases: { list: async () => [] },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => { throw new Error('shared unavailable'); } } } },
  } as never);
  assert.deepEqual(state, { status: 'error', message: 'shared unavailable' });
});

test('delete guard fires exactly one DELETE per confirmed action', async () => {
  let calls = 0;
  const client = { knowledgeBases: { remove: async () => { calls += 1; } } } as never;
  const guard = createDeleteGuard(client);
  const [first, second] = await Promise.all([guard.confirm('kb-1'), guard.confirm('kb-1')]);
  assert.equal(first, 'deleted');
  assert.equal(second, 'in-flight', 'double confirm must not fire a second DELETE');
  assert.equal(calls, 1);
  assert.equal(await guard.confirm('kb-1'), 'deleted', 'guard releases after the first delete settles');
  assert.equal(calls, 2);
});