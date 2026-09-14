import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, supportedLocales } from '@weknora/i18n';

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
