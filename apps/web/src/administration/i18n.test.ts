import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, supportedLocales } from '@weknora/i18n';

test('administration UI copy resolves through the shared Vue-derived locale catalog', () => {
  const keys = [
    'mobileAdministration.loadFailed',
    'mobileAdministration.members',
    'mobileAdministration.invite',
    'mobileAdministration.role.owner',
    'mobileAdministration.expires',
    'settings.navGroups.systemAdministration',
    'settings.system',
  ];
  for (const locale of supportedLocales) {
    for (const key of keys) {
      const value = formatMessage(locale, key, { count: 1, date: '2026-09-15', tenant: 1, role: 'viewer', name: 'Ada' });
      assert.notEqual(value, key, `${locale} is missing ${key}`);
    }
  }
});
