import { test } from 'node:test';
import assert from 'node:assert/strict';
import { formatMessage, messages, supportedLocales } from '../src/index.ts';

// The auth-page copy must stay byte-identical to the Vue baseline extraction.
const SAMPLE_KEYS = [
  'platform.subtitle',
  'auth.loginHint',
  'auth.usernameInvalid',
  'auth.passwordMustContainSpecialChar',
  'inviteRegister.bannerTitle',
  'language.languageSaved',
  'auth.workspaceOnboarding.create',
  'auth.workspaceOnboarding.inviteOnlyNotice',
  'tenant.create.nameLabel',
  'tenant.create.nameRequired',
  'tenantInvitation.myInbox.acceptButton',
  'tenantInvitation.myInbox.declineSuccess',
];

test('auth message keys exist in every supported locale', () => {
  for (const locale of supportedLocales) {
    for (const key of SAMPLE_KEYS) {
      assert.notEqual(formatMessage(locale, key), key, `${locale} missing ${key}`);
    }
  }
});

test('auth key sets are identical across locales', () => {
  const keySets = supportedLocales.map((locale) => Object.keys(messages[locale]).sort());
  for (let i = 1; i < keySets.length; i++) {
    assert.deepEqual(keySets[i], keySets[0]);
  }
});

test('formatMessage interpolates {name} params for invite banner (Vue bannerTitle)', () => {
  const zh = formatMessage('zh-CN', 'inviteRegister.bannerTitle', { tenant: '产品部' });
  assert.ok(zh.includes('产品部'));
});
