import assert from 'node:assert/strict';
import test from 'node:test';

import { buildInviteLink, sharedResourceRow, type SharedResourceRow } from './settings-actions.ts';

// Vue baseline: frontend/src/views/organization/OrganizationSettingsModal.vue
// (invite-code generation + copy) and the org shares list, whose rows carry
// id (share id), knowledge_base_id and knowledge_base_name
// (internal/handler/organization.go ListOrgShares).

test('invite links carry the code on the current page path', () => {
  const link = buildInviteLink('abc123', { origin: 'https://app.example.com', pathname: '/platform/organizations', search: '?section=x' });
  const url = new URL(link);
  assert.equal(url.origin + url.pathname, 'https://app.example.com/platform/organizations');
  assert.equal(url.searchParams.get('section'), 'x');
  assert.equal(url.searchParams.get('invite_code'), 'abc123');
});

test('invite links preserve other query parameters', () => {
  const link = buildInviteLink('abc', { origin: 'https://a.dev', pathname: '/o', search: '?tab=2&invite_code=stale' });
  const url = new URL(link);
  assert.equal(url.searchParams.get('tab'), '2');
  assert.equal(url.searchParams.get('invite_code'), 'abc');
});

test('shared resource rows expose the ids needed to unshare', () => {
  const row: SharedResourceRow = sharedResourceRow({ id: 'share-1', knowledge_base_id: 'kb-9', knowledge_base_name: 'Docs', permission: 'view' });
  assert.equal(row.shareId, 'share-1');
  assert.equal(row.knowledgeBaseId, 'kb-9');
  assert.equal(row.name, 'Docs');
  assert.equal(row.permission, 'view');
});

test('shared resource rows tolerate alternate key spellings', () => {
  const row = sharedResourceRow({ share_id: 's2', name: 'FAQ base' });
  assert.equal(row.shareId, 's2');
  assert.equal(row.knowledgeBaseId, '');
  assert.equal(row.name, 'FAQ base');
  assert.equal(row.permission, '');
});

test('unshare is only offered when both ids exist', () => {
  assert.equal(sharedResourceRow({ id: 's', knowledge_base_id: 'k' }).canUnshare, true);
  assert.equal(sharedResourceRow({ id: 's' }).canUnshare, false);
  assert.equal(sharedResourceRow({}).canUnshare, false);
});
