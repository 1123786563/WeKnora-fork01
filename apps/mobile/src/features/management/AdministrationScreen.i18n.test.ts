import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, supportedLocales } from '@weknora/i18n';
import { administrationAuditActionKey } from './administration.ts';

const keys = [
  'mobileAdministration.noWorkspace', 'mobileAdministration.loadFailed', 'mobileAdministration.someDataUnavailable',
  'mobileAdministration.back', 'mobileAdministration.title', 'mobileAdministration.refresh', 'mobileAdministration.role',
  'mobileAdministration.roleUnavailable', 'mobileAdministration.tenant', 'mobileAdministration.manageApiKeys',
  'mobileAdministration.readOnly', 'mobileAdministration.loading', 'mobileAdministration.members', 'mobileAdministration.noMembers',
  'mobileAdministration.remove', 'mobileAdministration.removeTitle', 'mobileAdministration.removeMessage', 'mobileAdministration.cancel',
  'mobileAdministration.invite', 'mobileAdministration.inviteEmail', 'mobileAdministration.sendInvitation', 'mobileAdministration.sending',
  'mobileAdministration.openInvitations', 'mobileAdministration.noPendingInvitations', 'mobileAdministration.expires',
  'mobileAdministration.revoke', 'mobileAdministration.auditLog', 'mobileAdministration.noAuditEntries', 'mobileAdministration.emailRequired',
  'mobileAdministration.ownerInviteForbidden', 'mobileAdministration.unableToSend', 'mobileAdministration.unableToUpdateRole',
  'mobileAdministration.unableToRemove', 'mobileAdministration.unableToRevoke', 'mobileAdministration.role.admin',
  'mobileAdministration.role.contributor', 'mobileAdministration.role.viewer', 'mobileAdministration.status.active', 'mobileAdministration.status.pending',
] as const;

test('administration screen copy resolves in every supported locale', () => {
  for (const locale of supportedLocales) {
    for (const key of keys) assert.notEqual(formatMessage(locale, key), key, `${locale}:${key}`);
    assert.match(formatMessage(locale, 'mobileAdministration.members', { count: 2 }), /2|２|２人|2명|2/);
    assert.notEqual(formatMessage(locale, 'mobileAdministration.removeMessage', { name: 'Ada' }), 'mobileAdministration.removeMessage');
  }
});

test('audit labels follow locale changes and keep server enum values intact when unknown', () => {
  const success = supportedLocales.map((locale) => formatMessage(locale, 'mobileAdministration.audit.outcome.success'));
  for (const label of success) assert.notEqual(label, 'mobileAdministration.audit.outcome.success');
  assert.equal(formatMessage('zh-CN', 'mobileAdministration.audit.action.rbac.invitation_sent'), '已发送邀请');
  assert.equal(formatMessage('en-US', 'mobileAdministration.audit.action.rbac.invitation_sent'), 'Invitation sent');
  assert.equal(formatMessage('ja-JP', 'mobileAdministration.audit.actor.owner'), 'オーナー');
  assert.equal(formatMessage('ko-KR', 'mobileAdministration.audit.outcome.denied'), '거부됨');
  assert.equal(formatMessage('ru-RU', 'mobileAdministration.audit.actor.system_admin'), 'Системный администратор');
  assert.equal(administrationAuditActionKey('future.audit.action'), null);
});
