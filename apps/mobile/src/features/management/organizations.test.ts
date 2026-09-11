import assert from 'node:assert/strict';
import test from 'node:test';
import { canManageOrganization, validateOrganizationDraft } from './organizations.ts';

test('mobile organization writes are limited to organization admins', () => {
  assert.equal(canManageOrganization({ my_role: 'admin' }), true);
  assert.equal(canManageOrganization({ my_role: 'viewer' }), false);
  assert.equal(canManageOrganization({ owner_id: 'owner-1' }), false);
});

test('mobile organization creation validates the server-owned name boundary', () => {
  assert.deepEqual(validateOrganizationDraft(' ', 'description'), ['Name is required']);
  assert.deepEqual(validateOrganizationDraft('Research', 'description'), []);
});
