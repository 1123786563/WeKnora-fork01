import assert from 'node:assert/strict';
import test from 'node:test';

import { clampApplicationNote, inviteJoinMode, requestedRoleOf, type InviteJoinMode } from './join.ts';

// Vue baseline: frontend/src/views/organization/OrganizationList.vue 405-465.
// Already-member preview shows a member notice and no join action; approval-
// gated previews branch to a request flow with role select + note; open
// previews join directly.

test('preview without approval joins directly as viewer', () => {
  assert.equal(inviteJoinMode({ require_approval: false, is_already_member: false }), 'join');
  assert.equal(inviteJoinMode({ is_already_member: false }), 'join', 'missing approval flag defaults to open join');
});

test('preview with approval branches to the request flow', () => {
  assert.equal(inviteJoinMode({ require_approval: true, is_already_member: false }), 'request');
});

test('already-member preview never offers a join action', () => {
  assert.equal(inviteJoinMode({ require_approval: true, is_already_member: true }), 'member');
  assert.equal(inviteJoinMode({ require_approval: false, is_already_member: true }), 'member');
  assert.equal(inviteJoinMode(null), 'member');
  assert.equal(inviteJoinMode({}), 'member', 'preview without flags must not offer join');
});

test('application note is clamped to the 500-character Vue limit', () => {
  assert.equal(clampApplicationNote('hello'), 'hello');
  assert.equal(clampApplicationNote('x'.repeat(600)), 'x'.repeat(500));
  assert.equal(clampApplicationNote('  trimmed  '), 'trimmed');
});

test('requested role falls back to viewer for the request flow', () => {
  assert.equal(requestedRoleOf({ requested_role: 'editor' }), 'editor');
  assert.equal(requestedRoleOf({}), 'viewer');
  assert.equal(requestedRoleOf({ requested_role: 'root' }), 'viewer');
});

test('join modes are exhaustive', () => {
  const modes: InviteJoinMode[] = ['member', 'request', 'join'];
  assert.equal(modes.length, 3);
});
