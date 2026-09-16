import assert from 'node:assert/strict';
import test from 'node:test';

import { organizationRoleLabel, organizationSettingsSections } from './summary.ts';

test('keeps organization roles explicit for list and detail surfaces', () => {
  assert.equal(organizationRoleLabel('admin'), 'Admin');
  assert.equal(organizationRoleLabel('viewer'), 'Viewer');
  assert.equal(organizationRoleLabel('unknown'), 'unknown');
});

test('hides join-request settings from non-admin organization members like Vue', () => {
  assert.deepEqual(organizationSettingsSections('edit', false), ['basic', 'members', 'shares', 'agents', 'invite']);
  assert.deepEqual(organizationSettingsSections('edit', true), ['basic', 'members', 'requests', 'shares', 'agents', 'invite']);
  assert.deepEqual(organizationSettingsSections('create', false), ['basic', 'permissions']);
});
