import assert from 'node:assert/strict';
import test from 'node:test';

import { wikiEditPermission, findSharedKBGrant } from './edit-permission.ts';
import type { KBSurfaceKB, KBSurfaceMe } from '../knowledge/permissions.ts';

const me = (overrides: Record<string, unknown> = {}): KBSurfaceMe => ({
  user: { id: 'user-1' },
  memberships: [{ tenant_id: 7, role: 'viewer' }],
  tenant: { id: 7 },
  ...overrides,
} as KBSurfaceMe);

const kb = (overrides: Record<string, unknown> = {}): KBSurfaceKB => ({
  id: 'kb-1',
  creator_id: '',
  ...overrides,
} as KBSurfaceKB);

// ---- Vue 契约（frontend/src/views/knowledge/KnowledgeBase.vue:288-293 canEdit）----
// isViaShare ? 共享 grant ∈ {admin, editor}
// : isOwner(creator_id === me.id) ? true
// : hasRole('admin')（当前租户角色 ≥ admin）? true
// : canEditKB → 共享 grant ∈ {admin, editor}

test('parity editor: org-share grant editor unlocks the wiki edit entry (isViaShare branch)', () => {
  const shared = [{ knowledge_base: { id: 'kb-1' }, permission: 'editor' }];
  assert.equal(wikiEditPermission(kb(), me({ memberships: [{ tenant_id: 7, role: 'viewer' }] }), shared), true);
});

test('parity admin: org-share grant admin unlocks the wiki edit entry', () => {
  const shared = [{ knowledge_base: { id: 'kb-1' }, permission: 'admin' }];
  assert.equal(wikiEditPermission(kb(), me(), shared), true);
});

test('read-only share grant hides the wiki edit entry even for a local tenant admin (isViaShare guard)', () => {
  const shared = [{ knowledge_base: { id: 'kb-1' }, permission: 'viewer' }];
  const tenantAdmin = me({ memberships: [{ tenant_id: 7, role: 'admin' }] });
  assert.equal(wikiEditPermission(kb({ creator_id: 'user-1' }), tenantAdmin, shared), false);
});

test('home-tenant KB: active-tenant admin membership unlocks the edit entry without any share record', () => {
  const tenantAdmin = me({ memberships: [{ tenant_id: 7, role: 'admin' }] });
  assert.equal(wikiEditPermission(kb(), tenantAdmin, []), true);
  assert.equal(wikiEditPermission(kb(), tenantAdmin, null), true);
});

test('home-tenant KB: active-tenant owner membership (role level >= admin) unlocks the edit entry', () => {
  const tenantOwner = me({ memberships: [{ tenant_id: 7, role: 'owner' }] });
  assert.equal(wikiEditPermission(kb(), tenantOwner, null), true);
});

test('tenant admin in a non-active tenant does not unlock the edit entry (currentTenantRole semantics)', () => {
  const otherTenantAdmin = me({ memberships: [{ tenant_id: 9, role: 'admin' }], tenant: { id: 7 } });
  assert.equal(wikiEditPermission(kb(), otherTenantAdmin, null), false);
});

test('KB creator is the owner regardless of tenant role (backend emits creator_id, not user_id)', () => {
  const viewer = me({ memberships: [{ tenant_id: 7, role: 'viewer' }] });
  assert.equal(wikiEditPermission(kb({ creator_id: 'user-1' }), viewer, null), true);
  // Vue isOwner: empty creator_id (legacy KB) never counts as owner.
  assert.equal(wikiEditPermission(kb({ creator_id: '' }), viewer, null), false);
});

test('tenant contributor alone does not grant wiki edit on someone else\'s KB (Vue hasRole parity)', () => {
  const contributor = me({ memberships: [{ tenant_id: 7, role: 'contributor' }] });
  assert.equal(wikiEditPermission(kb(), contributor, null), false);
});

test('row-level my_permission mirrors the backend share grant for cross-tenant access', () => {
  assert.equal(wikiEditPermission(kb({ my_permission: 'editor' }), me(), null), true);
  assert.equal(wikiEditPermission(kb({ my_permission: 'viewer' }), me(), null), false);
});

test('plain viewer without grant, creator claim, or admin role sees no edit entry', () => {
  assert.equal(wikiEditPermission(kb(), me(), null), false);
  assert.equal(wikiEditPermission(kb(), null, null), false);
});

test('findSharedKBGrant matches by knowledge_base id and normalizes the permission', () => {
  const rows = [
    { knowledge_base: { id: 'kb-other' }, permission: 'editor' },
    { knowledge_base: { id: 'kb-1' }, permission: ' Admin ' },
  ];
  assert.deepEqual(findSharedKBGrant(rows, 'kb-1'), { permission: 'admin' });
  assert.equal(findSharedKBGrant(rows, 'kb-missing'), null);
  assert.equal(findSharedKBGrant(undefined, 'kb-1'), null);
  assert.equal(findSharedKBGrant([{ knowledge_base: null, permission: 'editor' }], 'kb-1'), null);
});
