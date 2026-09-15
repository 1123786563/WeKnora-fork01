import assert from 'node:assert/strict';
import test from 'node:test';

import { knowledgeBaseDetailPath, loadKnowledgeBases, saveKnowledgeBase } from './list.ts';

test('loads real client data into a renderable list state', async () => {
  const state = await loadKnowledgeBases({
    knowledgeBases: {
      list: async () => [{ id: 'kb-1', name: 'Docs' }],
    },
  } as never);

  assert.deepEqual(state, { status: 'success', items: [{ id: 'kb-1', name: 'Docs' }] });
});

test('uses the Vue detail route for initialized knowledge-base navigation', () => {
  assert.equal(knowledgeBaseDetailPath('kb/one'), '/platform/knowledge-bases/kb%2Fone');
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
  const { deleteKnowledgeBase } = await import('./list.ts');
  assert.deepEqual(await saveKnowledgeBase(client, null, { name: 'FAQ', type: 'faq', summary_model_id: 'llm-1' }), { id: 'kb-2', name: 'FAQ' });
  await saveKnowledgeBase(client, 'kb-2', { name: 'FAQ v2', summary_model_id: 'llm-1' });
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

test('enforces the Vue editor name and description limits at the save boundary', async () => {
  const client = { knowledgeBases: { create: async () => ({ id: 'kb-1' }) } } as never;
  await assert.rejects(saveKnowledgeBase(client, null, { name: 'n'.repeat(51) }), /50 characters/);
  await assert.rejects(saveKnowledgeBase(client, null, { name: 'Valid', description: 'd'.repeat(201) }), /200 characters/);
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

test('matches Vue editor submit validation before calling the mutation', async () => {
  let calls = 0;
  const client = { knowledgeBases: {
    create: async () => { calls += 1; return { id: 'kb-3' }; },
    update: async () => { calls += 1; return { id: 'kb-3' }; },
  } } as never;

  await assert.rejects(
    saveKnowledgeBase(client, null, { name: 'Docs', type: 'document', summary_model_id: 'llm-1', indexing_strategy: { vector_enabled: true, keyword_enabled: true } }),
    /embedding model is required/,
  );
  await assert.rejects(
    saveKnowledgeBase(client, null, { name: 'Docs', type: 'document', embedding_model_id: 'embed-1', indexing_strategy: { vector_enabled: true, keyword_enabled: true } }),
    /summary model is required/,
  );
  await assert.rejects(
    saveKnowledgeBase(client, null, { name: 'Docs', type: 'document', embedding_model_id: 'embed-1', summary_model_id: 'llm-1', indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: false, graph_enabled: false } }),
    /at least one indexing strategy/,
  );
  await assert.rejects(
    saveKnowledgeBase(client, null, { name: 'Docs', type: 'document', embedding_model_id: 'embed-1', summary_model_id: 'llm-1', vlm_config: { enabled: true, model_id: '' }, indexing_strategy: { vector_enabled: true, keyword_enabled: true } }),
    /multimodal model is required/,
  );
  await assert.rejects(saveKnowledgeBase(client, null, { name: 'x'.repeat(51), type: 'faq', summary_model_id: 'llm-1' }), /50 characters/);
  await assert.rejects(saveKnowledgeBase(client, null, { name: 'Docs', type: 'faq', summary_model_id: 'llm-1', description: 'x'.repeat(201) }), /200 characters/);
  assert.equal(calls, 0, 'invalid Vue submissions must not call the API');
});

test('preserves forbidden error codes for the no-permission branch', async () => {
  const state = await loadKnowledgeBases({
    knowledgeBases: { list: async () => { throw Object.assign(new Error('No access'), { code: 'TENANT_FORBIDDEN' }); } },
  } as never);

  assert.deepEqual(state, { status: 'error', code: 'TENANT_FORBIDDEN', message: 'No access' });
});
