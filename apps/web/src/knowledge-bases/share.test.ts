import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildShareRequest,
  getShareableOrganizations,
  loadKnowledgeBaseShares,
  mutateKnowledgeBaseShare,
  reduceShareDialog,
  type ShareDialogState,
} from './share.ts';

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

test('only exposes owner, admin, and editor organizations that are not already shared', () => {
  assert.deepEqual(getShareableOrganizations([
    { id: 'owner', name: 'Owner', is_owner: true },
    { id: 'admin', name: 'Admin', my_role: 'admin' },
    { id: 'editor', name: 'Editor', my_role: 'editor' },
    { id: 'viewer', name: 'Viewer', my_role: 'viewer' },
    { id: 'shared', name: 'Already shared', my_role: 'editor' },
  ], [{ organization_id: 'shared' }]), [
    { id: 'owner', name: 'Owner', is_owner: true },
    { id: 'admin', name: 'Admin', my_role: 'admin' },
    { id: 'editor', name: 'Editor', my_role: 'editor' },
  ]);
});

test('opens with no organization and viewer permission, then preserves the selected permission', () => {
  const opened = reduceShareDialog(undefined, { type: 'open' });
  assert.deepEqual(opened, { open: true, organizationId: '', permission: 'viewer', status: 'idle' });
  assert.deepEqual(reduceShareDialog(opened, { type: 'select-organization', organizationId: 'org-1' }), {
    ...opened, organizationId: 'org-1',
  });
  assert.deepEqual(reduceShareDialog(opened, { type: 'set-permission', permission: 'editor' }), {
    ...opened, permission: 'editor',
  });
});

test('blocks submission without share permission or organization and exposes submitting state', () => {
  const initial = reduceShareDialog(undefined, { type: 'open' });
  assert.deepEqual(reduceShareDialog(initial, { type: 'submit', canShare: false }), {
    ...initial, status: 'forbidden', message: 'You do not have permission to share',
  });
  const selected = reduceShareDialog(initial, { type: 'select-organization', organizationId: 'org-1' });
  assert.deepEqual(reduceShareDialog(selected, { type: 'submit', canShare: true }), {
    ...selected, status: 'submitting',
  });
});

test('success closes and resets so reopening starts a fresh share', () => {
  const selected: ShareDialogState = { open: true, organizationId: 'org-1', permission: 'editor', status: 'submitting' };
  const success = reduceShareDialog(selected, { type: 'success' });
  assert.deepEqual(success, { open: false, organizationId: '', permission: 'viewer', status: 'success' });
  assert.deepEqual(reduceShareDialog(success, { type: 'open' }), {
    open: true, organizationId: '', permission: 'viewer', status: 'idle',
  });
});

test('cancel and Escape close without submitting, while rejected requests remain retryable', () => {
  const selected: ShareDialogState = { open: true, organizationId: 'org-1', permission: 'viewer', status: 'idle' };
  assert.equal(reduceShareDialog(selected, { type: 'cancel' }).open, false);
  assert.equal(reduceShareDialog(selected, { type: 'escape' }).open, false);
  const rejected = reduceShareDialog({ ...selected, status: 'submitting' }, { type: 'failure', message: 'share denied' });
  assert.deepEqual(rejected, { ...selected, status: 'error', message: 'share denied' });
});
