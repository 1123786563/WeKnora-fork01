import assert from 'node:assert/strict';
import test from 'node:test';

import { formatMessage } from '@weknora/i18n';

// S00 N-4: onboarding create dialog and JoinPage previously hardcoded English
// (Cancel / Creating… / Loading… / Close / Join workspace / form labels).
// Every copy string consumed by those surfaces must resolve to a localized
// message for both supported locales instead of falling back to the raw key.
const keys = [
  'auth.workspaceOnboarding.creating',
  'auth.workspaceOnboarding.cancel',
  'auth.workspaceOnboarding.close',
  'auth.workspaceOnboarding.loadingInvitations',
  'auth.workspaceOnboarding.workspaceCreationFailed',
  'auth.workspaceOnboarding.workspaceFallback',
  'auth.join.title',
  'auth.join.checkingInvitation',
  'auth.join.invitationMissingToken',
  'auth.join.invitationInvalid',
  'auth.join.joinPrefix',
  'auth.join.asRole',
  'auth.join.workspaceFallback',
  'auth.join.username',
  'auth.join.email',
  'auth.join.password',
  'auth.join.confirmPassword',
  'auth.join.creatingAccount',
  'auth.join.createAccountAndJoin',
  'auth.join.invitationRegistrationFailed',
];

test('join and onboarding copy keys resolve for zh-CN and en-US (S00 N-4)', () => {
  for (const key of keys) {
    for (const locale of ['zh-CN', 'en-US'] as const) {
      const rendered = formatMessage(locale, key, { id: 't1', role: 'owner' });
      assert.notEqual(rendered, key, `${locale} must localize ${key}`);
      assert.ok(rendered.length > 0, `${locale} value for ${key} must be non-empty`);
    }
  }
});

test('join intro interpolation renders tenant and role (S00 N-4)', () => {
  const zh = formatMessage('zh-CN', 'auth.join.asRole', { role: 'owner' });
  assert.ok(zh.includes('owner'), 'zh-CN asRole must keep the role token');
  const en = formatMessage('en-US', 'auth.join.asRole', { role: 'owner' });
  assert.ok(en.includes('owner'), 'en-US asRole must keep the role token');
});
