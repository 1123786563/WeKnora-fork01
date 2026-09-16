import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, supportedLocales } from '../src/index.ts';

const keys = ['mobileAuth.invitationToken', 'mobileAuth.invitationExpires', 'mobileAuth.workspacesTitle', 'mobileAuth.back', 'mobileAuth.workspacesEmpty', 'mobileAuth.restoringSession', 'mobileAuth.loadingWorkspaces', 'mobileAuth.current', 'mobileAuth.switching', 'mobileAuth.loadWorkspacesFailed', 'mobileAuth.switchWorkspaceFailed', 'mobileAuth.serverTitle', 'mobileAuth.serverHint', 'mobileAuth.serverPlaceholder', 'mobileAuth.saveServer', 'mobileAuth.savingServer', 'mobileAuth.serverSaveFailed'];

test('mobile auth catalog contains every key in every supported locale', () => {
  for (const locale of supportedLocales) {
    for (const key of keys) {
      const value = formatMessage(locale, key);
      assert.notEqual(value, key, `${locale} missing ${key}`);
      assert.ok(value.trim(), `${locale} has empty ${key}`);
    }
  }
});

test('mobile auth invitation expiry copy interpolates per locale', () => {
  for (const locale of supportedLocales) {
    assert.match(formatMessage(locale, 'mobileAuth.invitationExpires', { expiresAt: '2030-01-01' }), /2030-01-01/);
  }
});
