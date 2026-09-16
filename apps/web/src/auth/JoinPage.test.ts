import assert from 'node:assert/strict';
import test from 'node:test';

import { readInviteToken } from './join.ts';

test('reads only the invitation token from the join URL', () => {
  assert.equal(readInviteToken('?token=invite%2Fone&next=%2Fplatform'), 'invite/one');
  assert.equal(readInviteToken('?next=%2Fplatform'), '');
});
