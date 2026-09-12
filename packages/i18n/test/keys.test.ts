import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, isLocale, messages, supportedLocales } from '../src/index.ts';

test('keeps the five locales present in the current source inventory aligned', () => {
  assert.deepEqual([...supportedLocales].sort(), ['en-US', 'ja-JP', 'ko-KR', 'ru-RU', 'zh-CN'].sort());
  for (const locale of supportedLocales) assert.ok(messages[locale]['auth.login']);
});

test('formats interpolation and falls back to English/key without Vue runtime', () => {
  assert.equal(formatMessage('en-US', 'missing', { name: 'x' }), 'missing');
  // Vue baseline (frontend/src/i18n/locales/en-US.ts auth.login) is authoritative.
  assert.equal(formatMessage('en-US', 'auth.login'), 'Login');
  assert.equal(isLocale('fr-FR'), false);
});

test('keeps the React foundation keys and placeholders aligned across every locale', () => {
  const requiredKeys = ['auth.login', 'auth.email', 'auth.password', 'common.loading', 'common.retry', 'common.cancel', 'common.workspaceRequired', 'common.itemCount'];
  for (const locale of supportedLocales) {
    for (const key of requiredKeys) assert.ok(messages[locale][key], `${locale} is missing ${key}`);
  }
  assert.equal(formatMessage('en-US', 'common.itemCount', { count: 3 }), '3 items');
  assert.equal(formatMessage('zh-CN', 'common.itemCount', { count: 3 }), '3 项');
  const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
  for (const key of requiredKeys) {
    const expected = placeholders(messages['en-US'][key]);
    for (const locale of supportedLocales) assert.deepEqual(placeholders(messages[locale][key]), expected, `${locale} placeholder mismatch for ${key}`);
  }
});
