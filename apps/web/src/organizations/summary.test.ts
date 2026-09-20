import assert from 'node:assert/strict';
import test from 'node:test';

import { organizationRoleLabel, organizationSettingsNavGroups, organizationSettingsSections } from './summary.ts';

test('keeps organization roles explicit for list and detail surfaces', () => {
  assert.equal(organizationRoleLabel('admin'), 'Admin');
  assert.equal(organizationRoleLabel('viewer'), 'Viewer');
  assert.equal(organizationRoleLabel('unknown'), 'unknown');
});

test('hides join-request settings from non-admin organization members like Vue', () => {
  assert.deepEqual(organizationSettingsSections('edit', false), ['basic', 'members', 'shares', 'agents']);
  assert.deepEqual(organizationSettingsSections('edit', true), ['basic', 'members', 'requests', 'shares', 'agents']);
  assert.deepEqual(organizationSettingsSections('create', false), ['basic', 'permissions']);
});

// R487 K1: Vue OrganizationSettingsModal.vue navGroups (L1028-1058) renders the
// edit-mode navigation as THREE titled groups — 基础[基本信息] /
// 成员与协作[成员管理, 加入申请] / 共享资源[共享知识库, 共享智能体]. The invite
// affordances live inside the basic 邀请成员 card, never as a nav item.
test('edit-mode navigation groups mirror the Vue three titled groups without an invite item', () => {
  assert.deepEqual(organizationSettingsNavGroups('edit', true), [
    { key: 'basic', titleKey: 'organization.navGroups.basic', items: ['basic'] },
    { key: 'management', titleKey: 'organization.navGroups.management', items: ['members', 'requests'] },
    { key: 'resources', titleKey: 'organization.navGroups.resources', items: ['shares', 'agents'] },
  ]);
  // Non-admins lose only the admin-gated join-request entry (Vue isAdmin gate).
  assert.deepEqual(organizationSettingsNavGroups('edit', false), [
    { key: 'basic', titleKey: 'organization.navGroups.basic', items: ['basic'] },
    { key: 'management', titleKey: 'organization.navGroups.management', items: ['members'] },
    { key: 'resources', titleKey: 'organization.navGroups.resources', items: ['shares', 'agents'] },
  ]);
});

test('create-mode navigation keeps the single Vue 基础 group', () => {
  assert.deepEqual(organizationSettingsNavGroups('create', true), [
    { key: 'basic', titleKey: 'organization.navGroups.basic', items: ['basic', 'permissions'] },
  ]);
});
