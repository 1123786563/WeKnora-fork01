import assert from 'node:assert/strict';
import test from 'node:test';

import { roleLabel, invitationIsOpen } from './summary.ts';

test('uses explicit tenant roles and invitation terminal states', () => {
  assert.equal(roleLabel('owner'), 'Owner');
  assert.equal(roleLabel('viewer'), 'Viewer');
  assert.equal(roleLabel('future-role'), 'future-role');
  assert.equal(invitationIsOpen('pending'), true);
  assert.equal(invitationIsOpen('expired'), false);
});
