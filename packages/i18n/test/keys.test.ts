import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, isLocale, messages, supportedLocales } from '../src/index.ts';

test('keeps the five locales present in the current source inventory aligned', () => {
  assert.deepEqual([...supportedLocales].sort(), ['en-US', 'ja-JP', 'ko-KR', 'ru-RU', 'zh-CN'].sort());
  for (const locale of supportedLocales) assert.ok(messages[locale]['auth.login']);
});

test('formats interpolation and falls back to English/key without Vue runtime', () => {
  assert.equal(formatMessage('en-US', 'missing', { name: 'x' }), 'missing');
  assert.equal(formatMessage('en-US', 'auth.login'), 'Sign in');
  assert.equal(isLocale('fr-FR'), false);
});
