import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, supportedLocales } from '@weknora/i18n';

const keys = [
  'organization.title', 'organization.empty', 'organization.createOrg', 'organization.name',
  'organization.namePlaceholder', 'organization.nameRequired', 'organization.description',
  'organization.descriptionPlaceholder', 'organization.noMembers', 'organization.members.listTitle',
  'organization.settings.membersDesc', 'organization.settings.permissionsIconHint',
  'organization.sharedResources.loading', 'organization.sharedResources.kbListTitle',
  'organization.sharedResources.agentListTitle', 'organization.settings.noSharedKB',
  'organization.settings.noSharedAgents', 'organization.joinRequests.listTitle',
  'organization.settings.noPendingRequests', 'organization.settings.approve',
  'organization.settings.reject', 'organization.settings.removeShareFromOrg',
  'organization.settings.removeShareConfirm', 'organization.settings.removeAgentShareConfirm',
  'organization.settings.removeShareFailed', 'organization.detail.removeMember',
  'organization.detail.removeMemberConfirm', 'organization.memberRemoveFailed',
  'organization.role.admin', 'organization.role.editor', 'organization.role.viewer',
  'organization.share.permissionEditable', 'organization.share.permissionReadonly',
  'organization.share.sharedFrom', 'organization.rbac.cannotCreate', 'organization.createFailed',
  'organization.roleUpdateFailed', 'organization.settings.reviewFailed', 'common.cancel',
  'common.empty', 'common.error', 'mobileManagement.back', 'mobileApiKeys.refresh',
] as const;

test('organization management copy resolves in every supported mobile locale', () => {
  for (const locale of supportedLocales) for (const key of keys) {
    const value = formatMessage(locale, key);
    assert.notEqual(value, key, `${locale}:${key}`);
    assert.ok(value.trim(), `${locale}:${key} is empty`);
  }
  assert.match(formatMessage('zh-CN', 'organization.detail.removeMemberConfirm', { name: '成员' }), /成员/);
  assert.match(formatMessage('en-US', 'organization.settings.removeShareConfirm', { name: 'Docs' }), /Docs/);
});
