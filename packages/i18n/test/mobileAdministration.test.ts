import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, mobileAdministrationMessages, supportedLocales } from '../src/index.ts';

// Key-count guard: every locale must carry the exact same key inventory for
// the administration domain (R444 added the Vue two-step invite confirm copy
// and the TenantMembers.vue pager copy).
const expectedKeyCount = Object.keys(mobileAdministrationMessages['en-US']).length;

test('mobile administration catalog keeps an identical key count in every locale', () => {
  for (const locale of supportedLocales) {
    assert.equal(Object.keys(mobileAdministrationMessages[locale]).length, expectedKeyCount, `${locale} key count drifted`);
  }
  assert.ok(expectedKeyCount > 0);
});

test('invite two-step confirm and pager copy resolve in every supported locale', () => {
  const keys = [
    'mobileAdministration.confirmInviteTitle',
    'mobileAdministration.confirmInviteBody',
    'mobileAdministration.confirmSend',
    'mobileAdministration.pager.total',
    'mobileAdministration.pager.sizePerPage',
    'mobileAdministration.pager.jumper',
    'mobileAdministration.pager.pageUnit',
    'mobileAdministration.back',
  ];
  for (const locale of supportedLocales) {
    for (const key of keys) {
      const value = formatMessage(locale, key, { email: 'a@b.c', role: 'admin', total: 45, size: 20 });
      assert.notEqual(value, key, `${locale} missing ${key}`);
      assert.ok(value.trim(), `${locale} has empty ${key}`);
    }
  }
});

test('confirm body interpolates email and role placeholders per locale', () => {
  for (const locale of supportedLocales) {
    assert.match(formatMessage(locale, 'mobileAdministration.confirmInviteBody', { email: 'a@b.c', role: 'admin' }), /a@b\.c/);
    assert.match(formatMessage(locale, 'mobileAdministration.pager.total', { total: 45 }), /45/);
  }
});
