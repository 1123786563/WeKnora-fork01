import assert from 'node:assert/strict';
import test from 'node:test';

import {
  describeKnowledgeBaseList,
  describeKnowledgeBaseDetailPermission,
  describeKnowledgeBasePermission,
  filterKnowledgeBases,
  loadKnowledgeBaseActivity,
  mergeKnowledgeBaseScopes,
  saveKnowledgeBase,
} from './states.ts';

test('distinguishes loading, empty, forbidden, and error list branches', () => {
  assert.equal(describeKnowledgeBaseList({ status: 'loading' }).kind, 'loading');
  assert.equal(describeKnowledgeBaseList({ status: 'success', items: [] }).kind, 'empty');
  assert.equal(describeKnowledgeBaseList({ status: 'error', code: 'FORBIDDEN', message: 'denied' }).kind, 'forbidden');
  const network = describeKnowledgeBaseList({ status: 'error', code: 'NETWORK', message: 'offline' });
  assert.equal(network.kind, 'error');
  if (network.kind === 'error') assert.equal(network.message, 'offline');
});

test('keeps shared viewers read-only while editors can manage the base', () => {
  assert.deepEqual(describeKnowledgeBasePermission('viewer'), { canView: true, canEdit: false, canShare: false });
  assert.deepEqual(describeKnowledgeBasePermission('editor'), { canView: true, canEdit: true, canShare: false });
  assert.deepEqual(describeKnowledgeBasePermission('owner'), { canView: true, canEdit: true, canShare: true });
});

test('does not submit a knowledge-base mutation twice while the first is pending', async () => {
  let resolve!: (value: { id: string }) => void;
  const pending = new Promise<{ id: string }>((done) => { resolve = done; });
  let calls = 0;
  const save = saveKnowledgeBase(async () => { calls += 1; return pending; });

  const first = save({ name: 'Docs' });
  const second = save({ name: 'Docs' });
  assert.equal(second, first);
  assert.equal(calls, 1);
  resolve({ id: 'kb-1' });
  assert.deepEqual(await first, { status: 'success', value: { id: 'kb-1' } });
});

test('normalizes activity loading, empty, and error without inventing rows', async () => {
  assert.deepEqual(await loadKnowledgeBaseActivity(async () => ({ data: [] })), { status: 'empty', rows: [] });
  assert.deepEqual(await loadKnowledgeBaseActivity(async () => { throw new Error('activity unavailable'); }), {
    status: 'error', message: 'activity unavailable', rows: [],
  });
});

test('merges owned and shared cards once, keeping owned precedence and shared ordering', () => {
  const items = mergeKnowledgeBaseScopes(
    [{ id: 'owned', name: 'Owned', is_pinned: false }],
    [
      { knowledge_base: { id: 'owned', name: 'Duplicate' }, permission: 'viewer' },
      { knowledge_base: { id: 'shared', name: 'Shared', is_pinned: true, pinned_at: '2026-01-01' }, permission: 'viewer', share_id: 's1' },
    ],
  );
  assert.deepEqual(items.map((item) => item.id), ['shared', 'owned']);
  assert.equal(items.find((item) => item.id === 'owned')?.name, 'Owned');
  assert.equal(items.find((item) => item.id === 'shared')?.permission, 'viewer');
});

test('filters list search across name, description, creator, and type without changing empty semantics', () => {
  const items = [
    { id: '1', name: 'Docs', description: 'Platform guide', type: 'document' },
    { id: '2', name: 'Support', description: 'Answers', creator_name: 'Lin', type: 'faq' },
  ];
  assert.deepEqual(filterKnowledgeBases(items, '  lin '), [items[1]]);
  assert.deepEqual(filterKnowledgeBases(items, ''), items);
  assert.deepEqual(filterKnowledgeBases(items, 'missing'), []);
});

test('enforces share-level detail permissions and download restrictions', () => {
  assert.deepEqual(describeKnowledgeBaseDetailPermission({ permission: 'viewer', viaShare: true, tenantRole: 'admin' }), {
    canView: true, canEdit: false, canShare: false, canDownload: false, canMutateDocuments: false,
  });
  assert.equal(describeKnowledgeBaseDetailPermission({ permission: 'editor', viaShare: true, tenantRole: 'viewer' }).canEdit, true);
  assert.equal(describeKnowledgeBaseDetailPermission({ isOwner: true, tenantRole: 'contributor' }).canDownload, true);
});
