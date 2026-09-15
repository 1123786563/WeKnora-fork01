import assert from 'node:assert/strict';
import test from 'node:test';

import { canManageTenant, canViewAudit, isEditableMember, invitationIsOpen, roleLabel, tenantRoleFromMemberships } from './summary.ts';

test('uses explicit tenant roles and invitation terminal states', () => {
  assert.equal(roleLabel('owner'), 'Owner');
  assert.equal(roleLabel('viewer'), 'Viewer');
  assert.equal(roleLabel('future-role'), 'future-role');
  assert.equal(invitationIsOpen('pending'), true);
  assert.equal(invitationIsOpen('expired'), false);
});

test('matches Vue tenant administration permissions', () => {
  assert.equal(canManageTenant('owner'), true);
  assert.equal(canManageTenant('admin'), true);
  assert.equal(canManageTenant('contributor'), false);
  assert.equal(canManageTenant('viewer'), false);
  assert.equal(canViewAudit('owner'), true);
  assert.equal(canViewAudit('admin'), true);
  assert.equal(canViewAudit('viewer'), false);
});

test('does not expose member mutations for the current user or owner', () => {
  assert.equal(isEditableMember({ user_id: 'u-1', role: 'admin' }, 'u-1'), false);
  assert.equal(isEditableMember({ user_id: 'u-2', role: 'owner' }, 'u-1'), false);
  assert.equal(isEditableMember({ user_id: 'u-2', role: 'viewer' }, 'u-1'), true);
});

test('resolves the active tenant role from auth memberships', () => {
  assert.equal(tenantRoleFromMemberships([{ tenant_id: 7, role: 'admin' }], 7), 'admin');
  assert.equal(tenantRoleFromMemberships([{ tenant_id: 8, role: 'owner' }], 7), undefined);
  assert.equal(tenantRoleFromMemberships([{ tenant_id: 7, role: 123 }], 7), undefined);
});
