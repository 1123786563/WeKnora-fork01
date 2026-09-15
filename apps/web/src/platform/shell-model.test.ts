import assert from 'node:assert/strict';
import test from 'node:test';
import { activePlatformNavId, filterCommandItems, nextCommandIndex, platformGeometry, platformUserMenuItems, tenantDisplayName, tenantInitial, userInitial } from './shell-model.ts';

test('Vue-compatible navigation selects the most specific route', () => {
  assert.equal(activePlatformNavId('/platform/knowledge-bases/kb-1'), 'knowledge-bases');
  assert.equal(activePlatformNavId('/platform/settings?section=tenant'), 'settings');
  assert.equal(activePlatformNavId('/platform/unknown'), null);
});

test('tenant identity has stable fallback and initial', () => {
  assert.equal(tenantDisplayName({ tenantId: '7', name: '  ' }), '#7');
  assert.equal(tenantInitial({ tenantId: '7', name: ' acme ' }), 'A');
});

test('geometry preserves Vue sidebar dimensions and content floor', () => {
  assert.deepEqual(platformGeometry(1440, false), { sidebarWidth: 260, contentWidth: 1180, contentMinWidth: 340, collapsed: false });
  assert.deepEqual(platformGeometry(500, true), { sidebarWidth: 60, contentWidth: 440, contentMinWidth: 540, collapsed: true });
});

test('command palette filters and wraps keyboard selection', () => {
  const items = [{ id: 'a', label: 'Knowledge bases', hint: '/platform/knowledge-bases', run() {} }, { id: 'b', label: 'Settings', run() {} }];
  assert.deepEqual(filterCommandItems(items, 'knowledge').map((item) => item.id), ['a']);
  assert.equal(nextCommandIndex(0, 2, -1), 1);
  assert.equal(nextCommandIndex(1, 2, 1), 0);
  assert.equal(nextCommandIndex(0, 0, 1), -1);
});

test('user menu preserves Vue account and workspace actions', () => {
  assert.deepEqual(platformUserMenuItems({ hasTenant: true, canManageMembers: true, canLogout: true }).map((item) => item.id), [
    'personal-settings', 'workspace-settings', 'members', 'all-settings', 'logout',
  ]);
  assert.equal(platformUserMenuItems({ hasTenant: false, canManageMembers: false, canLogout: false }).at(-1)?.id, 'all-settings');
  assert.equal(userInitial('  alice '), 'A');
  assert.equal(userInitial(''), '?');
});
