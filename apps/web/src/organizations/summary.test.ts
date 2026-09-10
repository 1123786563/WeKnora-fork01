import assert from 'node:assert/strict';
import test from 'node:test';

import { organizationRoleLabel } from './summary.ts';

test('keeps organization roles explicit for list and detail surfaces', () => {
  assert.equal(organizationRoleLabel('admin'), 'Admin');
  assert.equal(organizationRoleLabel('viewer'), 'Viewer');
  assert.equal(organizationRoleLabel('unknown'), 'unknown');
});
