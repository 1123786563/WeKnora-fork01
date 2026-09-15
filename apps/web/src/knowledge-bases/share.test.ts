import assert from 'node:assert/strict';
import test from 'node:test';

import { buildShareRequest, loadKnowledgeBaseShares, mutateKnowledgeBaseShare } from './share.ts';

test('builds the Vue share request with an explicit organization and permission', () => {
  assert.deepEqual(buildShareRequest({ organizationId: 'org-1', permission: 'viewer' }), {
    organization_id: 'org-1', permission: 'viewer',
  });
});

test('loads shares into empty and ready branches without fallback rows', async () => {
  assert.deepEqual(await loadKnowledgeBaseShares(async () => ({ shares: [], total: 0 })), { status: 'empty', shares: [] });
  assert.deepEqual(await loadKnowledgeBaseShares(async () => ({ data: { shares: [{ id: 'share-1' }], total: 1 } })), {
    status: 'ready', shares: [{ id: 'share-1' }],
  });
});

test('deduplicates a pending share mutation and reports failure', async () => {
  let reject!: (error: Error) => void;
  const pending = new Promise<unknown>((_, fail) => { reject = fail; });
  let calls = 0;
  const mutate = mutateKnowledgeBaseShare(async () => { calls += 1; return pending; });
  const first = mutate({ permission: 'editor' });
  const second = mutate({ permission: 'editor' });
  assert.equal(first, second);
  assert.equal(calls, 1);
  reject(new Error('share failed'));
  assert.deepEqual(await first, { status: 'error', message: 'share failed' });
});
