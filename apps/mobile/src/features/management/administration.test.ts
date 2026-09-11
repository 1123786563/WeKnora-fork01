import assert from 'node:assert/strict';
import test from 'node:test';
import { canManageTenant, isRemovableMember, validateInvite } from './administration.ts';

test('mobile administration permits only owner and admin writes', () => {
  assert.equal(canManageTenant('owner'), true);
  assert.equal(canManageTenant('admin'), true);
  assert.equal(canManageTenant('contributor'), false);
  assert.equal(canManageTenant('viewer'), false);
});

test('mobile administration never offers removal of the owner', () => {
  assert.equal(isRemovableMember({ role: 'owner', status: 'active' }), false);
  assert.equal(isRemovableMember({ role: 'admin', status: 'active' }), true);
});

test('mobile invitation validates email and role before a server write', () => {
  assert.deepEqual(validateInvite(' ', 'viewer'), ['Email is required']);
  assert.deepEqual(validateInvite('person@example.com', 'owner'), ['Owner invitations are not allowed']);
  assert.deepEqual(validateInvite('person@example.com', 'admin'), []);
});
